# 유틸리티 개선 실행계획

목표: 독립 Go 모듈 구조를 유지하면서 호환성, 입력 처리, 요청 로그, CLI 결함을 사용자 관점 회귀 테스트로 수정한다.
기준: 원격 main 및 작업 시작점 5c5415cab6cebb940571d9a53e47f868e8d5be6c.
작업 경로: C:/Users/magos/.codex/worktrees/utility-hardening/goproj
작업 브랜치: fix/utility-hardening
실행: superpowers:subagent-driven-development의 구현/검토 절차 및 실패 테스트 → 수정 → 성공 검증.

## 승인과 경계

- 긴 결합문자에서 JavaScript NFKC와 호환되지 않는 입력은 명시적 오류로 거절하도록 사용자 승인.
- workspace 설정과 scrypter/helloworld 의존성 변경만 승인. workflows 작업은 사용자 지시로 보류.
- 기존 checkout의 fix/jwttool, 미추적 jwttool/와 docs/는 건드리지 않는다.
- 후속 사용자 지시에 따라 모듈별 변경과 공통 문서·workspace 설정을 분리 커밋하고 push 및 PR 생성까지 수행한다.
- 공용 모듈을 신설하지 않고 기존 표준 라이브러리, Cobra, Echo를 재사용한다.

## 1. Scrypter

파일: scrypter/password/password.go, password_test.go, password/testdata/, main_test.go, README.md, go.mod, go.sum.

- [x] Node.js normalize("NFKC")와 crypto.scryptSync로 고정 fixture를 생성하고 Node/ICU/Unicode 버전 및 재생성 명령을 기록.
- [x] 공개 Hash/Verify 테스트: 30/31/40개 결합문자, 원래 U+034F 포함 입력, 잘못된 UTF-8. 수정 전 실패 확인.
- [x] UTF-8을 먼저 검사하고 NFKC가 추가한 U+034F를 감지하여 명시적으로 거절. 입력 U+034F는 삭제하지 않음.
- [x] Verify(nodeHash, 긴 결합문자)는 false,error, Hash는 빈 문자열,error. CLI는 mismatch(1)가 아닌 오류(2).
- [x] x/text v0.39.0 이상과 필요한 최소 Go 버전/간접 의존성만 갱신.
- [x] 지원 입력은 Node fixture와 일치하는지, test/vet/build/govulncheck 실행.

## 2. Hello World

파일: helloworld/main.go, main_test.go, README.md, go.mod, go.sum.

- [x] 실제 HTTP 요청 → handler → 상태 코드/로그를 확인하는 회귀 테스트 작성 및 실패 확인.
- [x] 기본 127.0.0.1:1323, --addr, 기본 본문 한도 1 MiB, --max-body-bytes, --log-body.
- [x] http.MaxBytesReader로 chunked/Content-Length 요청을 제한. 초과 413, 읽기/폼 파싱 실패 400.
- [x] Authorization, Proxy-Authorization, Cookie, Set-Cookie, X-API-Key 마스킹. 기본 query 값·본문·폼 값은 숨김.
- [x] --log-body 선택 시 제한된 본문을 재사용하여 urlencoded/multipart 파싱. multipart 임시 파일 제거.
- [x] Echo v4.16.0, x/net v0.61.0 및 필요한 간접 의존성 갱신. 표준 라이브러리 보안 수정 반영을 위해 최소 Go 1.27.3.
- [x] 경계 크기, 초과, 읽기 실패, 민감 헤더, 기본값, 원문 옵션, 폼 파싱 검증.

## 3. Jumin

파일: jumin/main.go, main_test.go, README.md.

- [x] 실제 CLI 입력과 기대 종료 코드/stdout/stderr를 표로 작성하고 실패 확인.
- [x] 날짜 정확히 1개, 유효 날짜, 1900–2099년, 성별 male/m/man/female/f/woman(대소문자 무관) 검사.
- [x] 표준 flag 유지. 옵션은 날짜 앞에만 허용. 날짜 뒤 옵션 및 추가 인자 거절.
- [x] 오류 stderr/종료 2, 도움말 종료 0.
- [x] 기존 생성/체크 숫자 보존, 1999/2000 남녀 경계와 체크 숫자 검증.
- [x] README 제목·사용법·성별·테스트 데이터 목적 정리. 실재/발급/비중복은 보장하지 않음.

## 4. Bcrypter 및 클립보드

파일: bcrypter/cmd/root.go, verify.go, 테스트, bcrypter/main_test.go, 두 모듈 cmd/clipboard*.go, scrypter/cmd/root.go, 각 README.

- [x] 생성 명령 RunE와 주입 가능한 비밀번호 입력/클립보드 출력. 결과 해시 stdout 한 줄, 프롬프트/오류 stderr.
- [x] 전역 명령 상태와 미사용 --toggle 제거. 기존 verify 출력/종료 계약 유지.
- [x] 생성 해시의 비밀번호 일치, 빈 값, 공백, 72/73바이트, 읽기/클립보드 실패 테스트.
- [x] bcrypt 접두어 $2a$, $2b$, $2y$만 허용. $2$, $2x$, 임의 접두어는 오류(2).
- [x] 모듈별 build-tag 경계로 Linux/macOS CGO=0 및 미지원 OS에서 클립보드 호출 전 명시적 오류.
- [x] Windows는 CGO 없이 기존 네이티브 구현 유지. OS/CGO/X11 요구사항 문서화.

## 5. 문서와 통합 검증

- [x] 루트 README: 네 모듈 목적, 설치, 지원 범위, 모듈별 test/vet/build 명령.
- [x] workspace/goproj.code-workspace에 scrypter/jumin 추가.
- [x] 네 모듈 test/vet/build, scrypter/helloworld govulncheck, 가능한 Linux CGO=0 검증.
- [x] 요구사항 검토 뒤 코드 품질 검토. 잔여 제한 및 실행하지 못한 검증 기록.
- [x] 원래 checkout 보존, 브랜치 및 diff 확인. CI 실행과 게시 상태는 별도로 보고.

## 검증 기록

### 재현 및 변경

- 수정 전 bcrypter 실제 CLI: $2z$, $2A$, $2$, $2x$ 허용 및 생성 입력 오류의 stdout 출력/종료1을 재현.
- 수정 전 scrypter: Node fixture의 긴 결합문자 불일치(false,nil), 원본 CGJ와 삽입된 CGJ의 혼동, 잘못된 UTF-8 수용을 재현.
- 수정 전 jumin: 입력 누락/잘못된 날짜/성별/연도/추가 인자가 성공 종료0으로 처리됨을 실제 실행 파일로 재현.
- 수정 전 helloworld: 본문 제한·읽기 실패·로그 노출·폼 파싱 및 잘못된 설정 검증 부재를 재현.
- 검토 중 GET/DELETE의 잘못된 Content-Type도 400이 되도록 회귀 테스트 후 수정. URL-encoded 본문 파싱은 표준 ParseForm의 POST/PUT/PATCH 범위 유지.
- 수정 후 요구사항 검토 및 별도 코드 품질 검토: 네 모듈 모두 통과, 미해결 지적사항 없음.
- 최종 GET/DELETE 수정은 Windows test/vet/build와 Go 1.27.3으로 빌드한 Linux CGO=0 테스트 바이너리 실행까지 재검증.

### 검증 환경과 결과

| 항목 | 환경 | 결과 |
| --- | --- | --- |
| 네 모듈 test / vet / build | Windows amd64, Go 1.27.0 (helloworld 1.27.3) | 통과 |
| 네 모듈 test / vet / build | Linux amd64, Go 1.27.3, CGO_ENABLED=0 | 통과 |
| scrypter 고정 fixture | Node 22.14.0, ICU 76.1, Unicode 16.0 | 생성 스크립트 assert 및 Go API 검증 통과 |
| govulncheck v1.9.0 | 네 모듈, Windows | 도달 가능한 취약점 각각 0건 |
| 원래 checkout 상태 | fix/jwttool + 미추적 docs/, jwttool/ | 변경 없이 보존 |

실행 명령: 각 모듈에서 `go test -mod=readonly -count=1 -timeout=2m ./...`,
`go vet -mod=readonly ./...`, `go build -mod=readonly .`,
`go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`.

helloworld는 외부 의존성을 갱신한 뒤에도 기존 Go 1.27.0 표준 라이브러리에서
6건이 보고되었다. Go 1.27.3으로 재검사해 0건을 확인하고 go.mod에 최소 버전을 반영했다.
도달하지 않는 의존성/패키지 수준 경고는 남아 있으므로, 이 결과를 모든 의존성에
취약점이 전혀 없다는 의미로 해석하지 않는다.

참조: [Better Auth v1.3.34 구현](https://github.com/better-auth/better-auth/blob/v1.3.34/packages/better-auth/src/crypto/password.ts),
[Go x/text 보안 권고](https://pkg.go.dev/vuln/GO-2026-5970).

### 남겨 둔 범위

- 사용자 지시로 workflows 생성/수정을 보류. 원격 CI는 실행하지 않음.
- 실제 Windows/macOS/X11 클립보드 쓰기와 Linux CGO=1 환경은 실행하지 않음.
  주입된 쓰기 경계와 Linux CGO=0의 실제 명령 경로는 테스트함.
- JavaScript와 완전히 동일한 정규화를 구현하지 않음. stream-safe 삽입이 필요한 입력은 명시적으로 거절.
- 최초 구현 단계에서는 commit/push/PR 없이 보존했다. 후속 게시 단계는 아래 범위로 진행한다.


## 게시 계획

2026-10-21 후속 요청에 따라 기존 fix/utility-hardening 브랜치에서 아래 순서로 분리한다.
모듈의 README와 테스트·fixture·의존성 변경은 해당 모듈 커밋에 포함한다.

1. fix: scrypter 유니코드 검증 및 클립보드 오류 처리 개선
2. fix: helloworld 요청 크기 제한 및 안전한 로그 기본값 적용
3. fix: jumin 입력 검증 및 오류 종료 코드 수정
4. fix: bcrypter 생성 명령 및 해시 형식 검증 개선
5. docs: 도구 모음 안내 및 개선 계획 정리

마지막 커밋에는 루트 README, 이 계획 문서, 승인된 workspace 모듈 목록만 포함한다.
GPG 서명 없이 커밋하고 비강제 push 후 main 대상 PR을 생성한다.
workflows 생성·수정은 계속 보류한다. 기존 작업 checkout은 변경하지 않는다.
