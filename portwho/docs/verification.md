# portwho 검증 기록

작성일: 2026-10-22 (Asia/Seoul). 기준 명세: 0.2, JSON schema: 2.
브랜치: `fix/portwho`. 버전 출력: `portwho dev`.

독립 모듈과 세 OS의 실제 수집기 코드를 추가했다. **첫 릴리스 완료
상태는 아니다.** macOS 지원 빌드는 아직 SDK로 컴파일하지 못했으며
Linux arm64 실기·Ubuntu 24.04 기준 환경 검증도 남아 있다.
초기 검증에서는 커밋·push하지 않았다. 2026-10-23 별도 사용자 요청에 따라
변경분을 재검토하고 커밋·push를 진행한다. 원격 PR/릴리스 게시 및 공통
실행/CI 설정 변경은 이번 작업 범위에 포함하지 않는다.

## 2026-10-23 커밋 전 재검토

- CLI·소유자 집계·출력과 Windows/Linux/Darwin 수집기의 실제 호출 경로를
  다시 읽었다. 수정이 필요한 동작 결함은 발견하지 못했다.
- Windows 전체 `go test -mod=readonly -count=1 -timeout=2m ./...`,
  `go vet -mod=readonly ./...`가 통과했고 gofmt 미적용 파일은 없었다.
- 일반 사용자 Linux amd64 컨테이너에서 전체
  `go test -race -mod=readonly -count=1 -timeout=2m ./...`와
  `go vet -mod=readonly ./...`가 다시 통과했다.
- 아래 초기 검증 기록과 성능 측정은 2026-10-22 결과이다. Mac SDK/실기,
  Linux arm64 실기·Ubuntu 24.04 기준 환경 등 남은 릴리스 요건을 완료로
  바꾸지 않는다. 소스 push와 첫 릴리스 완료는 별도 상태이다.

## 실행 환경과 결과

| 대상 | 실제 확인한 환경 | 결과 |
| --- | --- | --- |
| Windows amd64 | Windows 11 Pro, NT 10.0.26300.0, Intel i7-13700K, Go 1.27.0 | 전체 테스트·vet·CGO=0 빌드·실제 CLI IPv4/IPv6 통과 |
| Linux amd64 | Docker Debian trixie, WSL2 kernel 6.6.87.2 x86_64, Go 1.27.1, uid/gid 1000, 기본 컨테이너 보안 정책 | 전체 테스트·vet·race·실제 CLI IPv4/IPv6 통과 |
| Linux arm64 | Windows에서 GOOS=linux GOARCH=arm64 CGO_ENABLED=0 | 빌드·platform vet 통과; 실기 실행 미검증 |
| Darwin amd64/arm64 cgo | 실행 가능한 Mac/SDK 없음 | 코드·native fault fixture 작성 및 두 단계 소스 검토; 컴파일·실행 미검증 |
| Darwin amd64/arm64 no-cgo | Windows 교차 컴파일 | platform 테스트 바이너리 생성 성공; 실행하지 않았으며 지원 배포물에서 제외 |
| 비지원 Linux 386 | amd64 Docker 커널의 32비트 실행 | 실제 help/version 0, 조회 JSON unsupported_platform 및 3 확인 |

Windows 샌드박스에서 IP Helper 조회와 테스트용 loopback 연결이 거부되었다.
샌드박스 밖 현재 사용자 실행으로 전체 테스트를 확인했다. Linux 검증은
`--network none --user 1000:1000` 컨테이너에 소스와 모듈 캐시를 읽기 전용으로
연결했다. 테스트가 만든 컨테이너는 정리했으며 기존 컨테이너/볼륨은 변경하지 않았다.

최종 확인 명령(모듈 안에서 실행):

```sh
go test -mod=readonly -count=1 -timeout=2m ./...
go vet -mod=readonly ./...
go test -race -mod=readonly -count=1 -timeout=2m ./... # Linux 컨테이너
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... # Windows 소스
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -mode=binary <linux-amd64-binary>
```

취약점 검사에서 호출되는 취약점은 발견되지 않았다. Windows 소스 검사는
미호출 package 1건/module 13건, Linux 바이너리 검사는 미호출 module 13건을
별도로 보고했다. 이는 모든 의존성·플랫폼이 무조건 취약점이 없다는 의미는 아니다.

## 기능 검증

공통 테스트는 실제 `cmd.Run(args, stdout, stderr, provider)`를 호출해
출력, 파싱한 JSON, 종료 코드, 수집/PID 연결/이미지 조회 호출을 검사한다.
실제 네이티브 테스트는 자식 프로세스가 OS 할당 포트에 리슨하고 실제
portwho 바이너리를 실행해 JSON/PID/`--all`, 이미지 경로와 조회 후 생존을 검사한다.
빈 결과는 고정 포트를 추정하지 않고 fixture로 검증한다.

| 수용 기준 | 검증 근거와 상태 |
| --- | --- |
| A01 | `TestQueryContract`: 포트·LISTEN 상태 필터 통과 |
| A02 | `TestQueryContract`: 여러 포트 OR 통과 |
| A03 | `TestEmptyAndPartialTable`: 표/JSON/PID 정상 빈 결과 통과 |
| A04 | `TestQueryContract`, 실제 `TestNativeCLI/*/all` 통과 |
| A05 | `TestQueryContract`: 같은 PID의 주소군별 endpoint 유지 통과 |
| A06 | 요청한 family만 Collect에 전달하고 다른 family not_requested 확인 |
| A07 | IPv4만 있는 fixture의 IPv6 빈 결과 통과 |
| A08 | `TestOutputModes`: 포트 앞·뒤 옵션 해석 통과 |
| A09 | `TestOutputModes`: 08080·8080 정규화/중복 제거/정렬 통과 |
| A10 | `TestOutputModes`: PID 전체 고유 숫자 정렬 통과 |
| A11 | PID 모드 Resolve 수행·ImagePath 생략 통과 |
| A12 | JSON 객체 한 개 파싱 후 EOF 확인 통과 |
| A13 | fixture 및 실제 CLI의 --path 통과 |
| A14 | path null·PATH 열 생략 통과 |
| A15 | 인수 없음: stdout 빈값·stderr 사용법·OS 조회 없음·2 통과 |
| A16 | 모든 명세 잘못된 포트 형태·overflow 입력 거부 통과 |
| A17 | 명세의 상호 배타 조합 거부 통과 |
| A18 | 미정의 flag·family 대소문자/값·값 누락 거부 통과 |
| A19 | help/version OS 조회 없음·버전 문자열 통과 |
| A20 | metadata access_denied 시 owner 유지 통과 |
| A21 | 일반 이미지 오류 시 endpoint/PID 유지 통과 |
| A22 | Windows 이미지 callback 오류 87 보존 및 공통 os_error 분류 통과 |
| A23 | `TestTableDiagnosticsAndEscaping`: 성공 family 표 보존·진단·3 통과 |
| A24 | JSON 부분 수집의 complete=false·성공 family 보존 통과 |
| A25 | 부분 수집의 PID stdout 0바이트·3 통과 |
| A26 | 실패+정상 빈 family를 정상 빈 결과로 표시하지 않음 통과 |
| A27 | IPv6 wildcard에서 IPv4를 만들지 않음 통과 |
| A28 | scope별 endpoint 분리와 정렬 통과 |
| A29 | 한글·공백 보존, 제어문자/방향 제어문자 표 이스케이프 통과 |
| A30 | stdout/stderr 오류·short write·도움말 출력 오류, 반복 보고 방지 통과 |
| A31 | fixture 및 실제 비지원 Linux 386의 unsupported/3 통과 |
| A32 | 실제 비지원 Linux 386의 help/version 0 통과 |
| A33 | Windows/Netlink 길이·개수·경계 오류 fixture 통과; Darwin C fixture 미실행 |
| A34 | Windows 5회/64MiB·이미지 32768 상한, Netlink 재시도 fixture 통과; Darwin fixture 미실행 |
| A35 | owners=[]·unresolved endpoint 보존·3 통과 |
| A36 | unresolved의 PID stdout 0바이트·3 통과 |
| A37 | 공통 복수 owner 및 실제 Linux 상속 FD 부모·자식 PID 확인 통과 |
| A38 | 실제 Linux 복수 FD 동일 PID 중복 제거 통과 |
| A39 | 공통 endpoint 합집합 및 실제 Linux REUSEPORT 원시 소켓 식별 유지 통과 |
| A40 | 같은 endpoint 일부 소켓 미매핑 partial·기존 PID 보존 통과 |
| A41 | resolved와 추가 가시성 limitation 양립 통과 |
| A42 | hidepid mountinfo·권한 거부 procfs fixture 통과; 실제 hidepid mount 미검증 |
| A43 | 아래 별도 두 컨테이너 실제 namespace 분리 검사 통과 |
| A44 | Linux proc fixture 미매핑 소켓 및 공통 unresolved 보존 통과 |
| A45 | Netlink 중단/잘린 wire/ERROR/DONE·재시도 폐기·실제 수신 deadline 통과; 실제 ENOBUFS/MSG_TRUNC 주입 미검증 |
| A46 | 공통 limitation 의미 통과; Darwin libproc EACCES native fixture 작성·미실행 |
| A47 | portable Darwin token/PCB/generation/endpoint 매칭 거부 통과; 실제 Darwin Resolve 변조 식별자 테스트 작성·미실행 |
| A48 | 공통 수집 실패 의미 통과; sysctl 실패와 성공 libproc가 공존하는 Darwin fixture 작성·미실행 |
| A49 | backend_unavailable 공통 출력 fixture 통과, Darwin no-cgo platform 테스트 교차 컴파일; 해당 OS 실행 미검증 |
| A50 | 다섯 runtime fixture를 Windows/Linux에서 실행해 동일 CLI 의미 통과; 다섯 네이티브 조합 실행 미완료 |

Linux의 `TestLinuxDoesNotMixAnotherNetworkNamespace`는 CAP_SYS_ADMIN 부재로
skip되었다. 별도로 권한 추가 없이 `--network none`인 컨테이너 두 개를
생성해 실제 바이너리를 확인했다. 리스너가 있는 쪽의 임시 포트 45217은
resolved/PID 1이었고, 다른 namespace에서는 complete=true·listeners=[]·
종료 1이었다. 두 결과의 관측 namespace를 섞지 않았으며 테스트 컨테이너를 종료했다.

## 성능 측정

실제 실행 파일을 모드별 30회 실행한 wall-clock 측정. p95는 정렬한
30개 표본의 29번째 값이다. 측정값이며 실시간 응답 보장이 아니다.

| 환경/부하 | 표 p95/max ms | JSON p95/max ms | PID p95/max ms |
| --- | --- | --- | --- |
| Windows 11 i7-13700K, 약 55 endpoint/29 owner PID, 전체 프로세스 약 466, FD scan 0(reported_owner) | 37.87 / 308.19 | 34.92 / 39.80 | 36.36 / 37.37 |
| Linux amd64 WSL2 컨테이너 uid 1000, 테스트 리스너 IPv4 50+IPv6 50, owner PID 1, 프로세스 1, 약 104 FD | 3.42 / 3.95 | 2.03 / 2.08 | 1.99 / 2.02 |

Windows는 관리자 role=false인 현재 사용자 권한과 기존 보안 정책, Linux는
Docker 기본 보안 정책에서 측정했다. Windows 개수는 최종 측정 직후 관측값이며
실행 중인 시스템에서 바뀔 수 있다. Linux arm64/macOS 성능, 경계 최대 부하, 더 강한 보안
정책에서의 측정은 남아 있다. 측정 중 메타데이터 접근 거부는 조회 성공과
구분해 stderr에서 소비했다.

## 로컬 빌드 산출물

Windows/Linux CGO=0 개발 바이너리와 SHA256SUMS:
`C:\Users\magos\AppData\Local\Temp\portwho-build-20261022`.
게시된 릴리스나 macOS 지원 검증 완료를 뜻하지 않는다.

| 파일 | SHA-256 |
| --- | --- |
| portwho-windows-amd64.exe | 1ff0518df5365a154a74e4182a2a3f5c56ae0560a7311b71b1cac4b7e2065028 |
| portwho-linux-amd64 | 1d92dcf383ac83b19c16cfceb449c89999eb3a3547f68b91890cfe7191af9992 |
| portwho-linux-arm64 | aaccdd2c05d673dc5ca683b5e265be0cdfe09e98265f893f1cdcf62a8eefec9e |

`checks/`의 Darwin no-cgo 테스트 바이너리·Linux 386 비지원 검증 파일은
지원 배포 산출물에 포함하지 않는다. macOS 두 지원 바이너리·체크섬은 아직 없다.
SHA256SUMS는 ASCII/LF이며 Linux에서 세 파일의 실제 SHA-256 일치를 확인했다.

## 남은 릴리스 요건

- Ubuntu 24.04 기준 환경, Linux arm64 하드웨어의 실제 일반 사용자 IPv4/IPv6
  실행·권한 제한·성능 확인. arm64 교차 빌드를 실제 지원 검증으로 대체하지 않는다.
- Intel/Apple Silicon macOS 14+에서 SDK·C compiler 빌드, C fault fixtures,
  실제 pcblist64↔libproc 결합, 다른 UID 접근 제한, link-local scope, 자원 해제,
  CLI·성능 확인 및 OS/kernel/SDK/CPU 기록.
- 실제 hidepid/강화 정책과 ENOBUFS/MSG_TRUNC 장애 주입 등 위에 구분한 미검증
  사례 보강. 다섯 필수 OS/CPU의 배포 바이너리·체크섬을 만들고 검증 결과와 함께 게시.

## 참고 소스

Windows 표/바이트 순서는 [GetExtendedTcpTable](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable),
[MIB_TCP6ROW_OWNER_PID](https://learn.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcp6row_owner_pid)를 확인했다.
Linux dump 계약은 [sock_diag(7)](https://man7.org/linux/man-pages/man7/sock_diag.7.html),
[Netlink kernel 문서](https://docs.kernel.org/userspace-api/netlink/intro.html)를 확인했다.
Apple 소스는 사용자 제공 토큰을 파일에 저장하지 않고 GitHub API로 읽었고
호출 프로세스 종료 시 메모리의 인증 헤더를 제거했다. 참조 XNU commit은
`f6217f891ac0bb64f3d375211650a4c1ff8ca1ea`이다. 토큰은 저장소 산출물에 없다.
