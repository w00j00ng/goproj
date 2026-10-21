package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("jumin", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: jumin [--gender=male|female] YYYY-MM-DD")
		flags.PrintDefaults()
	}
	gender := flags.String("gender", "male", "성별: male/m/man 또는 female/f/woman (날짜 앞에 지정)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "expected exactly one date; place options before YYYY-MM-DD")
		return 2
	}
	t, err := time.Parse("2006-01-02", flags.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "invalid date: use YYYY-MM-DD")
		return 2
	}
	if t.Year() < 1900 || t.Year() > 2099 {
		fmt.Fprintln(stderr, "supported years: 1900 through 2099")
		return 2
	}

	// 생년월일을 YYMMDD 형식으로 변환
	firstPart := t.Format("060102")

	// 랜덤 시드 초기화
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	genderDigit := 1
	switch strings.ToLower(*gender) {
	case "male", "m", "man":
	case "female", "f", "woman":
		genderDigit = 2
	default:
		fmt.Fprintln(stderr, "invalid gender: use male/m/man or female/f/woman")
		return 2
	}
	if t.Year() >= 2000 {
		genderDigit += 2
	}

	// 뒷자리 첫번째 숫자 설정 후, 나머지 5자리 임의 숫자 생성
	secondPartDigits := fmt.Sprintf("%d", genderDigit)
	for i := 0; i < 5; i++ {
		secondPartDigits += strconv.Itoa(r.Intn(10))
	}

	// 앞 12자리 (6자리 생년월일 + 6자리 뒷자리) 생성
	twelveDigits := firstPart + secondPartDigits

	// 체크섯 계산 (가중치: [2,3,4,5,6,7,8,9,2,3,4,5])
	weights := []int{2, 3, 4, 5, 6, 7, 8, 9, 2, 3, 4, 5}
	sum := 0
	for i, ch := range twelveDigits {
		digit, _ := strconv.Atoi(string(ch))
		sum += digit * weights[i]
	}
	remainder := sum % 11
	checkDigit := (11 - remainder) % 10

	// 최종 주민등록번호 생성 ("YYMMDD-XXXXXXX")
	fullSecondPart := secondPartDigits + strconv.Itoa(checkDigit)
	rrn := firstPart + "-" + fullSecondPart
	fmt.Fprintln(stdout, rrn)
	return 0
}
