package delta

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/w00j00ng/goproj/tdelta/internal/compare"
)

const (
	header           = "TDLT\x01"
	MaxEncoded       = 16 << 20
	maxCommands      = 32 << 20
	copyOp      byte = 1
	insertOp    byte = 2
)

type command struct {
	op             byte
	offset, length int
}

func Create(base, target []byte) ([]byte, error) {
	changes, err := compare.Find(base, target)
	if err != nil && !errors.Is(err, compare.ErrLimit) {
		return nil, err
	}
	var commands []command
	if err == nil {
		add := func(op byte, offset, length int) {
			if length == 0 {
				return
			}
			if n := len(commands); n > 0 {
				last := &commands[n-1]
				if last.op == op && last.offset+last.length == offset {
					last.length += length
					return
				}
			}
			commands = append(commands, command{op, offset, length})
		}
		cursor := 0
		for _, change := range changes {
			add(copyOp, cursor, change.Start-cursor)
			a, b := base[change.Start:change.End], target[change.NewStart:change.NewEnd]
			var prefix, suffix int
			m := min(len(a), len(b))
			for prefix < m && a[prefix] == b[prefix] {
				prefix++
			}
			for suffix < m-prefix && a[len(a)-suffix-1] == b[len(b)-suffix-1] {
				suffix++
			}
			add(copyOp, change.Start, prefix)
			add(insertOp, change.NewStart+prefix, len(b)-prefix-suffix)
			add(copyOp, change.End-suffix, suffix)
			cursor = change.End
		}
		add(copyOp, cursor, len(base)-cursor)
	}
	var scratch [1 + 2*binary.MaxVarintLen64]byte
	total := 0
	for _, c := range commands {
		total += len(c.encode(scratch[:0]))
		if c.op == insertOp {
			total += c.length
		}
	}
	whole := command{op: insertOp, length: len(target)}
	if len(target) > 0 && (err != nil || total > len(whole.encode(scratch[:0]))+len(target)) {
		commands = []command{whole}
	}

	var encoded bytes.Buffer
	h := binary.AppendUvarint([]byte(header), uint64(len(base)))
	h = binary.AppendUvarint(h, uint64(len(target)))
	a, b := sha256.Sum256(base), sha256.Sum256(target)
	encoded.Write(h)
	encoded.Write(a[:])
	encoded.Write(b[:])
	z := zlib.NewWriter(&encoded)
	var writeErr error
	for _, c := range commands {
		if _, writeErr = z.Write(c.encode(scratch[:0])); writeErr != nil {
			break
		}
		if c.op == insertOp {
			if _, writeErr = z.Write(target[c.offset : c.offset+c.length]); writeErr != nil {
				break
			}
		}
	}
	if err := errors.Join(writeErr, z.Close()); err != nil {
		return nil, err
	}
	if encoded.Len() > MaxEncoded {
		return nil, errors.New("encoded delta exceeds size limit")
	}
	return encoded.Bytes(), nil
}

func (c command) encode(dst []byte) []byte {
	dst = append(dst, c.op)
	if c.op == copyOp {
		dst = binary.AppendUvarint(dst, uint64(c.offset))
	}
	return binary.AppendUvarint(dst, uint64(c.length))
}

func Apply(base, encoded []byte) ([]byte, error) {
	if err := compare.Validate(base); err != nil {
		return nil, fmt.Errorf("base: %w", err)
	}
	if len(encoded) > MaxEncoded {
		return nil, errors.New("encoded delta exceeds size limit")
	}
	if !bytes.HasPrefix(encoded, []byte(header)) {
		return nil, errors.New("invalid delta header or unsupported version")
	}
	input := bytes.NewReader(encoded[len(header):])
	baseSize, err := binary.ReadUvarint(input)
	if err != nil || baseSize != uint64(len(base)) {
		return nil, errors.New("invalid delta base length or base does not match")
	}
	targetSize, err := binary.ReadUvarint(input)
	if err != nil || targetSize > compare.MaxText {
		return nil, errors.New("invalid delta target length")
	}
	var baseHash, targetHash [sha256.Size]byte
	if _, err := io.ReadFull(input, baseHash[:]); err != nil {
		return nil, fmt.Errorf("delta base hash: %w", err)
	}
	if _, err := io.ReadFull(input, targetHash[:]); err != nil {
		return nil, fmt.Errorf("delta target hash: %w", err)
	}
	if sha256.Sum256(base) != baseHash {
		return nil, errors.New("base does not match delta (SHA-256)")
	}
	z, err := zlib.NewReader(input)
	if err != nil {
		return nil, fmt.Errorf("delta zlib: %w", err)
	}
	target, readErr := applyCommands(base, int(targetSize), z)
	if err := errors.Join(readErr, z.Close()); err != nil {
		return nil, err
	}
	if input.Len() != 0 {
		return nil, errors.New("unexpected data after delta zlib stream")
	}
	if sha256.Sum256(target) != targetHash {
		return nil, errors.New("target does not match delta (SHA-256)")
	}
	if err := compare.Validate(target); err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}
	return target, nil
}

func applyCommands(base []byte, size int, z io.Reader) ([]byte, error) {
	limit := &io.LimitedReader{R: z, N: maxCommands + 1}
	r := bufio.NewReader(limit)
	result := make([]byte, size)
	pos := 0
	for {
		op, err := r.ReadByte()
		if limit.N == 0 {
			return nil, errors.New("decoded delta exceeds size limit")
		}
		if err == io.EOF {
			if pos != size {
				return nil, errors.New("delta result has incorrect length")
			}
			return result, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read delta command: %w", err)
		}
		var offset uint64
		switch op {
		case copyOp:
			offset, err = binary.ReadUvarint(r)
			if err != nil || offset > uint64(len(base)) {
				return nil, errors.New("invalid COPY offset")
			}
		case insertOp:
		default:
			return nil, fmt.Errorf("unknown delta command %d", op)
		}
		n, err := binary.ReadUvarint(r)
		if err != nil || n == 0 || n > uint64(size-pos) {
			return nil, errors.New("invalid delta command length")
		}
		dest := result[pos : pos+int(n)]
		if op == copyOp {
			if n > uint64(len(base))-offset {
				return nil, errors.New("COPY exceeds base length")
			}
			copy(dest, base[int(offset):int(offset)+int(n)])
		} else if _, err := io.ReadFull(r, dest); err != nil {
			return nil, fmt.Errorf("read INSERT data: %w", err)
		}
		pos += int(n)
	}
}
