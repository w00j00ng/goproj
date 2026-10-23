# portwho 0.2 구현 계획

기준: 사용자 제공 2026-10-22 명세 0.2. 독립 모듈을 새 `fix/portwho`
브랜치에 추가한다. 기존 도구 및 공통 실행 설정은 변경하지 않는다.
2026-10-23 사용자 요청에 따라 변경분 검토와 재검증 후 커밋·push를 수행한다.
원격 PR 생성과 릴리스 게시는 이번 작업 범위에 포함하지 않는다.

## 설계

Cobra CLI가 인수를 검증한 뒤 주입된 Provider를 호출한다. Provider는
요청한 주소군의 소켓 수집, 선택 소켓의 PID 연결, PID별 이미지 조회를
분리한다. 공통 서비스는 원시 소켓별 연결 상태를 보존한 뒤 endpoint로
집계하며 render는 표/JSON schema 2/PID 출력을 담당한다.

Windows는 IP Helper, Linux는 SOCK_DIAG와 procfs, macOS는 SDK 헤더를
사용하는 C shim으로 pcblist64와 libproc를 연결한다. 외부 명령 fallback은
추가하지 않는다. 비지원 빌드도 도움말/버전은 제공한다.

## 작업 및 검증

- [x] 공통 계약: `cmd/root_test.go`에서 실제 실행 함수에 인수와 fixture를
  입력하고 출력/JSON/종료 코드/호출 횟수를 검증한다. 먼저 실패를 확인한다.
- [x] `internal/inspect/{model,service}.go`, `internal/render/render.go`,
  `cmd/root.go`, `main.go`: 원시 소켓 보존, owner 집계, 메타데이터 캐시,
  출력 실패 처리. 위 사용자 시나리오 테스트를 통과시킨다.
- [x] Windows: native 테이블 디코딩, 버퍼 제한, owner PID, 이미지 조회.
  잘린 native 데이터 테스트와 실제 자식 IPv4/IPv6 리스너 조회를 수행한다.
- [x] Linux: Netlink multipart 검증/재시도/제한, procfs FD 연결/경합 감지.
  디코더 fixture와 Linux 실행 환경에서 가능한 네이티브 검증을 수행한다.
- [ ] Darwin: pcblist64/libproc token+generation 연결, scoped IPv6,
  cgo/no-cgo 구분 구현 및 소스 검토 완료. Mac SDK 컴파일·실행은 미완료.
- [x] README, 명세 수용 기준별 검증 기록, 현재 실행 가능한 모듈
  테스트/vet/빌드 및 Windows/Linux amd64 성능 측정.
- [ ] 첫 릴리스: Linux arm64 실기, Ubuntu 24.04 기준 환경, Intel/Apple
  Silicon Mac의 SDK 빌드·실행·권한 제한·성능 및 다섯 대상 배포 산출물.
  5개 네이티브 조합을 확인하기 전에는 릴리스 완료라고 선언하지 않는다.

## 검증 명령

모듈 디렉터리에서 `go test -count=1 -timeout=2m ./...`, `go vet ./...`,
`go build .`를 실행한다. Windows/Linux 빌드는 CGO_ENABLED=0,
Darwin 지원 빌드는 해당 macOS SDK와 CGO_ENABLED=1이 필요하다.
생성한 실행 파일은 저장소 밖 임시 디렉터리에 둔다.
