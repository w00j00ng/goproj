# portwho 프로그램 명세

- 명세 버전: 0.2
- 수정일: 2026-10-22 (Asia/Seoul)
- 대상: goproj의 신규 독립 Go CLI 모듈
- 대상 첫 릴리스: v0.1.0
- 상태: 구현 기준 제안안
- 실행 파일: Windows는 `portwho.exe`, Linux·macOS는 `portwho`
- 예정 모듈 경로: `github.com/w00j00ng/goproj/portwho`

## 1. 목적과 이번 개정

portwho는 실행 중인 운영체제의 로컬 TCP LISTEN 소켓을 조회하고, 해당 소켓과 연결이 확인된 PID 및 가능한 프로세스 이름·실행 이미지 경로를 출력하는 명령줄 프로그램이다.

대표 사용 방식은 세 OS에서 동일한 `portwho 8080`이다. 표 출력은 직접 확인에, JSON과 PID 출력은 개발 스크립트에 사용한다.

이번 개정에서 Windows 우선 지원 정책을 폐기한다. Windows·Linux·macOS의 실제 조회 기능을 첫 릴리스의 필수 범위로 둔다. 같은 소켓을 여러 프로세스가 공유하거나 리스너를 발견하고도 PID를 확인하지 못하는 경우를 표현하도록 결과 모델을 수정한다.

이 문서는 구현 계약이다. 프로그램의 구현·빌드·네이티브 실행 검증이 완료되었다는 뜻은 아니다. 명세 0.1의 단일 `pid` 구조는 `owners[]` 구조로 바뀌며 JSON 계약 버전은 2로 올린다.

## 2. 지원 범위

### 2.1 필수 배포 대상

| 운영체제 | 아키텍처 | 검증 기준 및 실행 요건 | 파일 |
| --- | --- | --- | --- |
| Windows | amd64 | Windows 11 | `portwho.exe` |
| Linux | amd64 | Ubuntu 24.04 LTS 기준, procfs와 NETLINK_SOCK_DIAG 사용 가능 | `portwho` |
| Linux | arm64 | 위와 같은 요건, arm64 실제 실행 검증 | `portwho` |
| macOS | amd64 | macOS 14 이상을 대상으로 Intel 환경에서 검증 | `portwho` |
| macOS | arm64 | macOS 14 이상을 대상으로 Apple Silicon에서 검증 | `portwho` |

위 OS 기준은 제품의 지원·검증 정책이다. API의 최소 지원 버전이나 아직 검증하지 않은 모든 미래 OS 버전의 동작을 보장한다는 뜻은 아니다. 릴리스 노트에 실제 시험한 OS 버전·커널·아키텍처·SDK를 기록한다.

Linux는 일반적인 커널 TCP 소켓과 procfs 환경을 대상으로 한다. 다른 배포판에서도 같은 실행 요건을 만족하면 사용할 수 있지만, 실제 검증 범위와 추정 호환 범위를 구분한다. Windows arm64와 32비트 OS는 첫 릴리스의 필수 대상에 넣지 않는다.

### 2.2 공통 기능

| 항목 | 첫 릴리스 계약 |
| --- | --- |
| 조회 대상 | 로컬 OS의 지원되는 커널 TCP 소켓 중 LISTEN 상태 |
| 주소군 | IPv4, IPv6, 둘 다 |
| 포트 선택 | 한 개 또는 여러 개의 숫자 포트, 전체 포트 |
| 핵심 결과 | 주소군, 로컬 주소, 포트, scope ID, 확인된 PID 목록 |
| 보조 결과 | 실행 이미지 이름, 선택적 전체 경로 |
| 출력 | 표, JSON 객체, 중복 없는 PID 목록 |
| 실행 방식 | 한 번 조회하고 종료 |
| 권한 | 현재 사용자 권한, 자동 권한 상승 없음 |
| 구성 | 독립 go.mod, OS별 실행 파일, 별도 설정 파일·DB 없음 |
| 외부 명령 | netstat·ss·lsof·tasklist·PowerShell 파싱을 기본 경로로 사용하지 않음 |

### 2.3 운영체제별 관측 범위

| OS | 소켓 범위 | PID 연결의 의미 |
| --- | --- | --- |
| Windows | Windows 네이티브 TCP owner 테이블 | API가 해당 행에 보고한 PID |
| Linux | 실행 프로세스의 현재 network namespace | 접근 가능한 procfs에서 해당 소켓의 FD 보유가 확인된 PID |
| macOS | BSD TCP 소켓 테이블 | libproc로 해당 소켓의 FD 보유가 확인된 PID |

`owners`는 플랫폼의 조회 방식으로 리스너와 연결이 확인된 PID 집합이다. 최초 소켓 생성자·최초 bind 수행자를 뜻하지 않는다. 모든 플랫폼에서 모든 상속 핸들·공유 FD 보유자를 빠짐없이 찾는다는 보장은 하지 않는다.

macOS 첫 구현의 `pcblist64` 경로는 BSD TCP 소켓을 대상으로 한다. Skywalk/userland 네트워킹 등 이 테이블 밖의 흐름까지 포함한 전체 macOS 네트워크 인벤토리로 표현하지 않는다. [S19][S27]

### 2.4 첫 릴리스에서 제공하지 않는 기능

- UDP, ESTABLISHED, TIME_WAIT 조회.
- 프로세스 종료·재시작·포트 강제 해제.
- 실제 bind를 시도하는 포트 사용 가능성 판정.
- 원격 호스트 검사, 포트 스캔, 서비스 응답 검사.
- 방화벽·예약 포트·포트 전달 설정 분석 또는 변경.
- 자동 namespace 전환, 컨테이너·WSL 내부 자동 탐색.
- 주기적 감시, TUI, 서비스 설치, 자동 관리자/root 재실행.
- 포트 범위·서비스 이름·호스트명 입력.
- 프로세스 명령행·환경변수·메모리 조회.

Linux·macOS를 미지원 stub만으로 제공하는 것은 이 명세를 충족하지 않는다.

## 3. 결과 모델과 성공의 의미

### 3.1 원시 소켓과 공개 endpoint를 구분한다

수집기 내부에서는 원시 소켓의 식별 정보를 유지한다. Linux는 inode·cookie, macOS는 socket/PCB opaque token·generation 등을 해당 실행 안에서 활용한다. Windows는 네이티브 행과 보고된 PID 관계를 보존한다.

PID 연결을 마친 뒤 공개 결과를 다음 키로 집계한다.

`(protocol, family, address, scope_id, port)`

공개 endpoint의 `owners[]`에는 확인된 PID를 중복 없이 모은다. 같은 소켓을 공유하는 두 PID가 모두 관찰되면 두 PID를 포함한다. 서로 다른 소켓이 같은 endpoint를 쓰는 경우도 공개 결과에서는 한 endpoint로 집계할 수 있다. 이 도구는 소켓 수나 FD 수를 세지 않는다.

서로 다른 원시 소켓을 PID 연결 전에 주소·포트만으로 합치면 안 된다. 같은 endpoint의 소켓 A는 PID가 확인되고 소켓 B는 미확인인 상황을 잃어버리기 때문이다.

### 3.2 PID 연결 상태

각 공개 endpoint에 `ownership_status`를 둔다.

| 상태 | 정확한 조건 | owners |
| --- | --- | --- |
| `resolved` | 해당 그룹의 모든 관찰 원시 소켓에 PID를 하나 이상 연결함 | 비어 있지 않음 |
| `partial` | 일부 원시 소켓에만 PID를 연결함 | 비어 있지 않음 |
| `unresolved` | 어느 원시 소켓에도 PID를 연결하지 못함 | 빈 배열 |

`resolved`는 각 관찰 소켓에 적어도 한 PID를 연결했다는 의미이다. 모든 실제 FD 보유자를 완전하게 열거했다거나 추가 소유자가 없다는 의미가 아니다.

예를 들어 공유 소켓에서 PID 100을 확인했으나 다른 PID의 FD를 읽지 못했다면 `resolved`일 수 있다. 확인된 가시성 제한은 `ownership.limitations`에 기록한다. 반면 같은 endpoint의 별도 소켓 두 개 중 하나만 매핑했으면 `partial`이다.

`owners=[]`는 PID 확인 불가이다. 소유 프로세스가 없다는 뜻이 아니며 PID 0·-1·null인 가짜 owner를 만들지 않는다. OS가 실제 보고한 PID 0은 가짜 값과 구분하여 정수 그대로 유지한다.

### 3.3 세 가지 실패를 구분한다

1. 소켓 목록 수집 실패: 어떤 리스너가 존재하는지 끝까지 읽지 못함.
2. PID 연결 미완료: 소켓은 발견했지만 그중 일부에 PID를 하나도 연결하지 못함.
3. 메타데이터 실패: PID는 확인했지만 실행 이미지 이름·경로를 읽지 못함.

1·2는 종료 코드 3의 원인이다. 3만 발생한 경우에는 핵심 조회 결과를 유지한다.

### 3.4 빈 결과와 시점

전체 소켓 수집이 성공한 빈 결과의 의미는 다음뿐이다.

> 요청한 범위의 TCP LISTEN 테이블에서 일치하는 항목을 찾지 못했다.

bind 가능 여부는 주소·주소군·옵션·다른 바인드 상태와 이후 변화에도 영향을 받으므로 `free`·`available`로 표시하지 않는다. [S7]

소켓 수집, PID 연결, 실행 이미지 조회는 별도 시점의 관측이다. PID·FD·inode·opaque token 재사용을 모든 경우에 탐지하거나 원자적인 소유 관계를 보장하지 않는다. 이미 관찰한 소켓은 PID·메타데이터 조회가 실패했다고 지우지 않는다.

## 4. CLI 계약

### 4.1 문법

```text
portwho <port> [<port>...] [--family both|ipv4|ipv6] [--json|--pids] [--path]
portwho --all            [--family both|ipv4|ipv6] [--json|--pids] [--path]

portwho --help
portwho --version
```

옵션은 포트 인수 앞뒤에 올 수 있다. stdin은 읽지 않는다. OS별로 다른 옵션이나 명령 이름을 만들지 않는다.

### 4.2 옵션

| 옵션 | 기본값 | 동작 |
| --- | --- | --- |
| `--all` | false | 포트 필터 없이 지원 범위의 TCP LISTEN 조회 |
| `--family` | `both` | `both`, `ipv4`, `ipv6` 중 선택 |
| `--json` | false | 표 대신 JSON 객체 하나 출력 |
| `--pids` | false | 표 대신 확인된 PID의 중복 없는 목록 출력 |
| `--path` | false | 실행 이미지 전체 경로를 출력에 포함 |
| `-h, --help` | — | 도움말 출력, OS 조회 없이 종료 |
| `--version` | — | `portwho <version>` 출력, OS 조회 없이 종료 |

`--pids`는 필터가 아니라 출력 모드이다. `--family` 값은 소문자만 허용한다.

### 4.3 포트 검증

- ASCII 숫자 `[0-9]+` 전체 일치, 십진수 1..65535.
- 앞자리 0 허용: `08080` → `8080`.
- 중복 제거 후 숫자 정렬.
- 인수 하나라도 잘못되면 OS 조회를 시작하지 않음.

| 입력 | 처리 |
| --- | --- |
| `8080` | 허용 |
| `8080 5432 8080` | `[5432, 8080]`으로 정규화 |
| `0`, `65536`, `-1`, `+80` | 오류 |
| `0x50`, `８０`, `80.0`, `" 8080 "` | 오류 |
| `80,443`, `8000-8100`, `http`, `localhost:8080` | 오류 |

### 4.4 조합과 사용 오류

- 포트 인수와 `--all`은 함께 사용할 수 없다.
- `--json`과 `--pids`는 함께 사용할 수 없다.
- `--path`와 `--pids`는 함께 사용할 수 없다.
- 일반 조회에 포트와 `--all`이 모두 없으면 오류이다.
- 도움말·버전 단독 요청은 OS 조회를 수행하지 않는다.
- 사용 오류는 stdout 없이 stderr에 이유·간단한 사용법을 출력하고 2로 종료한다.
- `--json`이 있어도 사용 오류를 조회 결과 JSON으로 포장하지 않는다.

다중 포트는 OR 조건이다. `8080 9090` 중 8080만 발견되었어도 수집·최소 PID 연결이 정상 완료되면 종료 코드는 0이다.

## 5. 수집·정규화·정렬

### 5.1 주소군

- `both`는 IPv4·IPv6를, 나머지는 지정한 주소군만 결과 대상으로 한다.
- Windows·Linux는 주소군별 수집을 수행한다.
- macOS의 공통 TCP 테이블 조회는 한 번으로 두 주소군을 포함할 수 있다. 반환 결과와 상태에는 요청한 주소군만 반영한다.
- 요청하지 않은 주소군은 `not_requested`이며 성공 판정에 영향을 주지 않는다.
- 요청한 주소군의 API 실패를 성공한 빈 결과로 바꾸지 않는다.
- IPv6 비활성이라는 추정만으로 errno를 무시하지 않는다. 정상 빈 dump와 API 실패를 구분한다.

### 5.2 주소 표현

- IPv4 wildcard는 `0.0.0.0`, IPv6 wildcard는 `::` 그대로 유지한다.
- IPv6 행을 근거로 IPv4 행을 생성하지 않는다.
- IPv4-mapped IPv6 주소도 실제 수집된 socket family를 유지한다.
- IPv6 scope ID를 보존하고 주소 문자열과 별도로 관리한다.
- `--family`는 socket family 필터이며 접속 가능성 판정이 아니다. [S6]

### 5.3 정렬·중복 제거

공개 endpoint 정렬은 포트 → IPv4/IPv6 → 주소 바이트 → scope ID 순서이다. endpoint 내부의 owners는 PID 숫자 오름차순으로 정렬하고 중복 제거한다.

동일 프로세스가 한 소켓을 여러 FD로 보유해도 owner 한 개이다. 같은 PID가 서로 다른 endpoint에 나타나는 것은 유지한다. `--pids`에서만 전체 결과의 PID를 합쳐 중복 제거한다.

## 6. 표·PID 출력 계약

### 6.1 공통

- stdout·stderr는 UTF-8, 프로그램 줄바꿈은 LF.
- ANSI 색상·진행 표시·자동 페이저 없음.
- 고정 안내·오류 문구는 영어.
- 유니코드 이름·경로 보존, 표의 제어문자 이스케이프.
- Unix의 비UTF-8 파일명 바이트는 유효 UTF-8 대체 문자로 출력하며 원본 바이트 복원을 보장하지 않음.
- 표 공백 폭은 자동화 계약이 아님. 자동화는 JSON 또는 PID 모드 사용.

### 6.2 기본 표

`portwho 8080`의 예시:

```text
PROTO  FAMILY  LOCAL             PID   PROCESS  OWNER
tcp    ipv4    127.0.0.1:8080    4321  backend  resolved
tcp    ipv4    127.0.0.1:8080    9876  worker   resolved
tcp    ipv6    [::1]:8080        ?     ?        unresolved
```

- 공개 endpoint의 확인된 owner마다 한 행을 표시한다.
- `owners=[]`이면 PID·PROCESS를 `?`로 표시한 행 하나를 남긴다.
- OWNER는 endpoint의 `ownership_status`이다. `partial`이어도 확인된 PID 행을 유지한다.
- `?`는 미확인 표식이며 PID 값이 아니다.
- 모든 결과가 LISTEN이므로 STATE 열은 생략한다.
- `--path`이면 PATH 열을 추가한다.
- IPv6 표기는 `[address%scope_id]:port`이다. scope가 0이면 생략한다.
- 이름·경로만 실패하면 `<access-denied>` 또는 `<unavailable>`을 사용한다.
- 메타데이터 경고는 표 모드에서 PID당 최대 한 번 stderr에 표시한다.
- 추가 owner 가시성에 관한 limitations가 있으면 표 모드에서 요약 경고 한 번을 출력할 수 있다. 명령 성공과 양립할 수 있음을 설명한다.

### 6.3 빈 결과

소켓 수집이 모두 성공하고 일치 endpoint가 없으면 표는 다음 한 줄을 stdout에 쓰고 1로 종료한다.

```text
No matching TCP listeners found.
```

수집 실패로 결과가 없는 경우 이 문장을 출력하지 않는다. stdout 없이 stderr에 수집 실패를 알리고 3으로 종료한다.

### 6.4 PID 모드

```text
4321
9876
```

- 확인된 PID만 숫자 정렬·중복 제거하여 한 줄에 하나씩 출력한다.
- 모든 실제 FD 보유자를 빠짐없이 반환한다는 의미는 아니다.
- 이름·경로 메타데이터 조회는 생략한다.
- Linux·macOS의 socket→PID 연결은 생략할 수 없다.
- 수집 완료·선택된 모든 원시 소켓의 최소 1PID 연결을 확인한 뒤 출력한다.
- 수집 실패 또는 `partial/unresolved` endpoint가 있으면 stdout은 0바이트, 종료 코드 3.
- 정상적인 빈 결과는 stdout 0바이트, 종료 코드 1.
- 출력 도중 장애가 나면 이미 출력한 바이트는 회수할 수 없으며 3으로 종료한다.

## 7. JSON 계약

### 7.1 정상 예시

Linux에서 같은 endpoint에 연결된 PID 두 개를 확인한 설명용 예시이다.

```json
{
  "schema_version": 2,
  "runtime": {
    "os": "linux",
    "arch": "amd64",
    "backend": "linux_sock_diag",
    "socket_scope": "kernel_tcp",
    "network_scope": "current_network_namespace",
    "pid_scope": "procfs_visible"
  },
  "query": {
    "ports": [8080],
    "family": "both",
    "include_path": false
  },
  "collection": {
    "started_at": "2026-10-22T06:00:00.000Z",
    "finished_at": "2026-10-22T06:00:00.018Z",
    "complete": true,
    "ipv4": "ok",
    "ipv6": "ok"
  },
  "ownership": {
    "mode": "best_effort",
    "method": "fd_holder",
    "limitations": []
  },
  "listeners": [
    {
      "protocol": "tcp",
      "family": "ipv4",
      "state": "LISTEN",
      "local": {
        "address": "127.0.0.1",
        "port": 8080,
        "scope_id": 0
      },
      "ownership_status": "resolved",
      "owners": [
        {
          "pid": 4321,
          "process": {
            "name": "backend",
            "path": null,
            "status": "ok",
            "reason": null
          }
        },
        {
          "pid": 9876,
          "process": {
            "name": "worker",
            "path": null,
            "status": "ok",
            "reason": null
          }
        }
      ]
    }
  ],
  "errors": []
}
```

### 7.2 필드 정의

| 필드 | 타입·값 및 의미 |
| --- | --- |
| `schema_version` | 정수 2. 프로그램·명세 버전과 별개 |
| `runtime.os` | Go GOOS 문자열. 필수 지원은 windows/linux/darwin |
| `runtime.arch` | Go GOARCH 문자열 |
| `runtime.backend` | `windows_iphelper / linux_sock_diag / darwin_pcblist64 / unsupported` |
| `runtime.socket_scope` | `kernel_tcp / bsd_tcp / unavailable` |
| `runtime.network_scope` | `host / current_network_namespace / unavailable` |
| `runtime.pid_scope` | `os_reported / procfs_visible / libproc_visible / unavailable` |
| `query.ports` | 정규화한 정수 배열. `--all`이면 null |
| `query.family` | `both / ipv4 / ipv6` |
| `query.include_path` | boolean |
| `collection.started_at, finished_at` | 소켓 수집 구간의 UTC RFC3339 시각, 최대 나노초 소수부, Z 종료 |
| `collection.complete` | 요청 주소군의 소켓 목록 수집이 모두 정상 완료되었는지 |
| `collection.ipv4, ipv6` | `ok / failed / not_requested` |
| `ownership.mode` | `best_effort` 고정 |
| `ownership.method` | `reported_owner / fd_holder / unavailable` |
| `ownership.limitations` | 확인된 가시성 제한 코드 배열. 없으면 [] |
| `listeners` | 정렬된 endpoint 배열. 없으면 [] |
| `listeners[].protocol` | `tcp` |
| `listeners[].family` | `ipv4 / ipv6` |
| `listeners[].state` | `LISTEN` |
| `listeners[].local.address` | 포트·대괄호·scope 접미사를 제외한 IP 문자열 |
| `listeners[].local.port` | 관찰한 정수 포트 |
| `listeners[].local.scope_id` | 정수. IPv4·scope 없는 IPv6는 0 |
| `listeners[].ownership_status` | `resolved / partial / unresolved` |
| `listeners[].owners` | 확인된 PID 및 메타데이터 배열. 미확인이면 [] |
| `listeners[].owners[].pid` | OS 또는 procfs 관점의 PID 정수 |
| `listeners[].owners[].process.name` | 실행 이미지 경로에서 얻은 이름 또는 null |
| `listeners[].owners[].process.path` | `--path`이고 조회 성공일 때 경로, 그 밖에는 null |
| `listeners[].owners[].process.status` | `ok / unavailable` |
| `listeners[].owners[].process.reason` | `null / access_denied / os_error` |
| `errors` | 핵심 조회·실행환경 오류 배열. 없으면 [] |

Windows runtime은 `kernel_tcp / host / os_reported`, Linux는 `kernel_tcp / current_network_namespace / procfs_visible`, macOS는 `bsd_tcp / host / libproc_visible`를 사용한다.

소켓 수집 시각은 PID 연결·프로세스 조회 완료 시각과 다를 수 있다. 원자적 스냅샷 시각으로 해석하지 않는다.

### 7.3 가시성 제한과 메타데이터

`ownership.limitations`는 중복 제거 후 코드 사전순으로 정렬하며 다음 값을 사용한다.

- `permission_denied`: 일부 PID/FD의 접근이 거부됨.
- `restricted_procfs`: hidepid 등 제한된 procfs 설정이 확인됨.
- `namespace_visibility`: 소켓 범위와 PID 관측 범위의 제한·불일치가 확인됨.
- `process_changed`: 조회 중 PID/FD/소켓 변경을 관찰함.
- `os_error`: owner 탐색 중 그 밖의 제한을 관찰함.

빈 limitations 배열은 숨겨진 owner가 없다는 증명이 아니다. 제한을 알고 있어도 선택된 각 원시 소켓에 PID 하나 이상을 연결하면 `resolved`일 수 있다. limitation만으로 종료 코드를 3으로 바꾸지 않는다.

`process.status=ok`이면 name은 문자열, reason은 null이다. `--path` 미지정 시 path=null이어도 정상이다. unavailable이면 name·path는 모두 null이다. 접근 거부로 확인된 경우만 access_denied를 사용하고 나머지는 os_error로 분류한다.

### 7.4 PID 연결 실패 예시

리스너 목록은 정상 수집했지만 소유 PID를 하나도 연결하지 못한 경우이다. 종료 코드는 3이다.

```json
{
  "schema_version": 2,
  "runtime": {
    "os": "darwin",
    "arch": "arm64",
    "backend": "darwin_pcblist64",
    "socket_scope": "bsd_tcp",
    "network_scope": "host",
    "pid_scope": "libproc_visible"
  },
  "query": {
    "ports": [5432],
    "family": "ipv4",
    "include_path": false
  },
  "collection": {
    "started_at": "2026-10-22T06:00:00.000Z",
    "finished_at": "2026-10-22T06:00:00.020Z",
    "complete": true,
    "ipv4": "ok",
    "ipv6": "not_requested"
  },
  "ownership": {
    "mode": "best_effort",
    "method": "fd_holder",
    "limitations": ["permission_denied"]
  },
  "listeners": [
    {
      "protocol": "tcp",
      "family": "ipv4",
      "state": "LISTEN",
      "local": {
        "address": "127.0.0.1",
        "port": 5432,
        "scope_id": 0
      },
      "ownership_status": "unresolved",
      "owners": []
    }
  ],
  "errors": [
    {
      "stage": "ownership",
      "family": null,
      "code": "ownership_unresolved",
      "os_code": null,
      "message": "Some observed listeners could not be associated with a PID."
    }
  ]
}
```

### 7.5 오류와 부분 결과

`errors[]`의 필드는 다음과 같다.

| 필드 | 값 |
| --- | --- |
| `stage` | `collection / ownership / platform` |
| `family` | `ipv4 / ipv6` 또는 null |
| `code` | `collection_failed / ownership_unresolved / unsupported_platform / backend_unavailable / resource_limit / invalid_os_data` |
| `os_code` | Windows 오류·Unix errno 숫자 또는 null |
| `message` | 사람이 읽는 영어 진단. 정확한 문장은 자동화 계약 아님 |

주소군 수집 실패는 해당 family를 failed로 표시하며 collection.complete=false이다. 성공한 다른 family의 행은 유지한다. 잘리거나 깨진 단일 family dump에서 얻은 일부 행은 완전한 결과처럼 사용하지 않는다.

macOS 공유 테이블 자체가 실패하면 요청한 모든 family를 failed로 표시한다. libproc의 개별 접근 거부는 소켓 목록 수집 실패로 바꾸지 않고 PID 연결 상태·limitations에 반영한다.

미매핑 소켓이 있으면 ownership 단계 오류를 한 번 요약하며, endpoint별 상태로 영향 범위를 표현한다. 알 수 없는 소유자가 실제로 특정 access denied PID라는 추측을 하지 않는다.

### 7.6 호환성·출력

- stdout은 BOM 없는 JSON 객체 하나와 LF.
- 필수 필드 유지, null과 [] 구분, 객체 키 순서는 비계약.
- 소비자는 알 수 없는 추가 필드를 무시할 수 있어야 한다.
- 필드 제거·타입·의미·호환되지 않는 enum 변경은 schema_version 증가.
- 표·경고·진행 로그를 JSON stdout에 섞지 않는다.
- 이름·경로 실패와 limitations는 JSON 필드로만 기록하며 stderr로 반복하지 않는다.
- collection/ownership/platform 핵심 오류는 JSON errors 및 stderr 짧은 진단에 기록한다.
- 출력 장치가 실패하면 완전한 JSON 문서를 보장할 수 없다.

## 8. 종료 코드

| 코드 | 조건 |
| --- | --- |
| `0` | 요청 소켓 목록 수집 성공, endpoint 있음, 모든 반환 endpoint가 resolved |
| `1` | 요청 소켓 목록 수집 성공, endpoint 없음 |
| `2` | 인수·옵션 오류, OS 조회 안 함 |
| `3` | 소켓 수집 실패, PID 연결 미완료, 실행환경·출력 실패 |

판정 순서:

1. 사용 오류이면 2.
2. 요청 소켓 수집이 불완전하면 3.
3. 수집이 완전하고 endpoint가 없으면 1.
4. 반환 endpoint에 partial 또는 unresolved가 있으면 3.
5. 그 밖에는 0.

이름·경로만 실패하거나 추가 owner 관측에 관한 limitation만 있으면 핵심 판정을 바꾸지 않는다. stdout 또는 stderr 쓰기 실패로 요청한 결과·진단을 완료하지 못하면 3으로 바꾼다. 같은 출력 스트림에 실패를 무한 재보고하지 않는다.

| 상황 | 표·JSON | --pids |
| --- | --- | --- |
| endpoint·PID 확인 | 결과, 0 | 확인된 PID, 0 |
| 이름만 실패 | PID 유지, 0 | 메타데이터 조회 생략, 0 |
| 관찰한 모든 소켓에 최소 1PID 연결, 추가 보유자 가시성 제한 | 결과와 limitation, 0 | 확인된 PID만, 0 |
| 소켓 발견·PID 미확인 | endpoint 유지, 3 | 빈 stdout, 3 |
| 일부 소켓에만 PID 연결 | 확인된 PID·미완료 상태, 3 | 빈 stdout, 3 |
| family 수집 실패 | 성공한 family의 결과 유지, 3 | 빈 stdout, 3 |
| 완전한 빈 결과 | 모드별 빈 결과, 1 | 빈 stdout, 1 |

도움말·버전은 출력 성공 시 0, 출력 실패 시 3이다. OS가 프로세스를 강제 종료한 경우는 정상 종료 코드 계약 밖이다.

## 9. 프로세스 이름·경로·권한

이름의 의미를 OS마다 바꾸지 않기 위해 첫 구현은 실행 이미지 경로에서 basename을 얻는다.

| OS | 경로 조회 |
| --- | --- |
| Windows | OpenProcess + QueryFullProcessImageNameW |
| Linux | /proc/<pid>/exe readlink |
| macOS | proc_pidpath |

`--path`는 전체 경로의 출력 여부를 제어한다. 내부에서 이름을 얻기 위해 경로 API를 호출할 수 있다. 기본 출력이나 오류 문구에 전체 경로를 몰래 추가하지 않는다.

Linux comm은 변경 가능하고 길이 제한이 있으므로 동일한 basename 의미의 대체 소스로 사용하지 않는다. 삭제된 이미지의 ` (deleted)` 표식 등 OS가 반환한 경로의 특성을 보존한다. 경로는 실행 이미지에 관한 관측 문자열이며 현재 접근 가능한 일반 파일의 존재를 보장하지 않는다. [S17][S28]

PID 연결과 이름 조회의 실패는 분리한다. 이미 PID가 확인되었으면 이름·경로를 얻지 못해도 owner를 제거하지 않는다. 자동 UAC/sudo 재실행, 디버그 권한 활성화, 시스템 보안 설정 변경을 하지 않는다.

Linux·macOS에서 사용자가 별도로 권한을 높여 실행할 수는 있지만, root가 모든 보안 정책·namespace 경계를 해소한다고 보장하지 않는다. 일반 사용자 실행도 필수 지원한다.

## 10. Windows 수집기

### 10.1 소켓 수집

GetExtendedTcpTable을 AF_INET·AF_INET6에 대해 TCP_TABLE_OWNER_PID_LISTENER로 호출한다. 이 조합은 두 family에서 제공된다. Reserved는 0이다. 반환 DWORD를 성공·오류 코드로 사용한다. [S1]

- 필요한 버퍼 크기를 읽고 할당한 뒤 데이터 획득.
- ERROR_INSUFFICIENT_BUFFER이면 갱신된 크기로 재시도.
- 최초 크기 확인 후 실제 획득은 family당 최대 5회.
- family당 단일 버퍼 상한 64 MiB.
- 상한·재시도 소진은 resource_limit.
- 실패 family의 행을 성공한 빈 결과로 변환하지 않음.

### 10.2 디코딩과 프로세스 조회

- Windows ABI의 구조체 크기·정렬·필드 오프셋을 사용.
- 포트, IPv4/IPv6 주소, IPv6 scope ID의 문서상 바이트 순서 처리.
- 항목 수·길이·오프셋·곱셈 overflow 검사.
- LISTEN의 원격 주소·포트는 공개 모델에서 제외. [S2][S3][S10]

프로세스 정보는 OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, pid), QueryFullProcessImageNameW(flags=0)로 읽는다. 열린 핸들은 모든 경로에서 닫는다. [S4][S5]

UTF-16 경로 버퍼는 문자 단위로 관리하며 최대 32,768 code unit으로 제한한다. ERROR_ACCESS_DENIED는 access_denied, 그 밖의 실패는 os_error이다. ERROR_INVALID_PARAMETER를 근거로 PID의 부재를 단정하지 않는다.

Windows의 ownership.method는 reported_owner이다. FD 보유자 전체 역조회는 추가하지 않는다.

## 11. Linux 수집기

### 11.1 소켓 열거

AF_NETLINK/NETLINK_SOCK_DIAG를 사용한다. family별 SOCK_DIAG_BY_FAMILY dump의 inet_diag_req_v2에 IPPROTO_TCP, TCP_LISTEN 상태 비트, NLM_F_REQUEST|NLM_F_DUMP를 지정한다. [S12]

- inet_diag_msg에서 주소·포트·state·inode·UID·interface index·cookie 획득.
- UID와 Netlink nlmsg_pid를 대상 프로세스 PID로 사용하지 않음.
- multipart 응답을 NLMSG_DONE까지 읽음.
- sequence·kernel sender·길이·정렬·NLMSG_ERROR·DONE 오류 검사.
- MSG_TRUNC·ENOBUFS·NLM_F_DUMP_INTR는 정상 dump가 아님.
- dump 재시도 최대 3회, 단일 수신 버퍼 64 MiB 상한.
- recv 대기는 dump 시도당 최대 3초로 제한하고 제한 초과는 수집 실패.
- 실패한 dump의 일부 행을 정상 결과로 사용하지 않음.

포트·주소는 UAPI의 __be16/__be32, inode·UID·interface·cookie 등은 일반 정수 필드로 해석한다. 전체 구조체에 같은 바이트 순서를 적용하지 않는다. [S15][S18]

/proc/net/tcp와 tcp6는 커널 문서에서 tcp_diag를 권장하며 deprecated로 설명하므로 첫 구현의 기본 수집 경로로 사용하지 않는다. Netlink 실패를 감춘 fallback도 두지 않는다. [S16]

### 11.2 소켓에서 PID로 연결

1. 선택 포트에 해당하는 원시 소켓의 inode·cookie를 유지한다.
2. 현재 procfs에서 관찰 가능한 PID를 열거한다.
3. /proc/<pid>/fd의 socket:[inode] 링크를 대상 inode 집합과 대조한다.
4. inode에 대해 확인된 PID 집합을 만들되 같은 PID의 복수 FD는 중복 제거한다.
5. 연결하지 못한 원시 소켓을 유지한다.
6. 연결 상태를 판정한 뒤 endpoint로 집계한다.

FD 링크 읽기에는 ptrace 접근 검사가 적용될 수 있다. hidepid는 PID 디렉터리 자체를 숨길 수 있다. 오류가 없었다는 이유만으로 모든 실제 소유자를 봤다고 주장하지 않는다. [S13][S14]

한 소켓을 여러 PID가 공유할 수 있으며, 서로 다른 SO_REUSEPORT 소켓도 주소·포트가 같을 수 있다. inode·cookie를 유지한 상태에서 각각 처리한다. 추가 보유자 열거는 best effort이다. [S29][S30]

권한 거부·제한된 procfs·namespace 제약은 확인된 범위만 limitations에 기록한다. 이미 선택 소켓마다 PID를 하나 이상 연결했다면 무관한 PID의 접근 거부만으로 resolved를 뒤집지 않는다. 미매핑 소켓이 남으면 partial/unresolved로 처리한다.

### 11.3 경합과 namespace

- /proc/<pid>/net은 해당 PID만의 소켓 목록이 아니다. network namespace의 정보를 가리킨다. [S31]
- 현재 network namespace를 자동 변경하지 않는다. namespace는 소켓·포트 공간을 분리한다. [S32]
- procfs의 PID 관측 범위를 host 전체 PID 범위라고 가정하지 않는다.
- PID·inode·cookie를 실행 간에 캐시하지 않는다.
- PID 디렉터리 핸들·시작 시각 등 사용 가능한 값으로 감지한 재사용은 해당 연결을 폐기한다.
- FD 링크에는 cookie가 없으므로 모든 inode 재사용 경합을 제거했다고 주장하지 않는다.
- ENOENT 등만으로 프로세스 종료를 단정하지 않고 관찰 실패로 분류한다.

소켓 목록이 완전하고 선택 포트 결과가 없으면 PID 탐색 없이 종료 1이 가능하다. procfs 부재·권한 제한 때문에 소켓 결과 자체를 삭제하지 않는다.

## 12. macOS 수집기

### 12.1 커널 소켓 목록

sysctlbyname("net.inet.tcp.pcblist64")를 사용하여 BSD TCP 소켓 테이블을 읽고 LISTEN과 요청 family·port를 선택한다. native xtcpcb64/xsocket64 레이아웃은 macOS SDK 헤더를 통해 취급한다. [S19]

- 필요한 길이 확인 → 버퍼 할당 → 읽기.
- 길이 변화에 대한 실제 획득 재시도 최대 5회, 단일 버퍼 상한 64 MiB.
- xig/xt 레코드 길이·버퍼 경계·generation을 검증.
- 시작·끝 generation 변화로 일관성이 깨졌으면 제한된 재수집.
- 정상적인 0개 테이블의 헤더 형태와 종료 레코드 여부는 지원 SDK/OS 동작대로 검증.
- 공유 테이블 실패는 요청한 모든 family의 수집 실패.
- BSD 테이블 밖의 userland/Skywalk 흐름은 첫 버전의 조회 범위에 포함하지 않음.

PID별 FD 스캔만으로 전체 소켓 목록을 대신하지 않는다. 그렇게 하면 다른 사용자 프로세스의 FD를 읽지 못했을 때 리스너 존재 자체가 결과에서 사라질 수 있다.

### 12.2 libproc와 PID 연결

- proc_listpids(PROC_ALL_PIDS)로 보이는 PID 목록 획득.
- proc_pidinfo(PROC_PIDLISTFDS)로 FD 목록 획득.
- socket FD에 proc_pidfdinfo(PROC_PIDFDSOCKETINFO) 호출.
- TCP·LISTEN·family 정보를 검증한 뒤 커널 테이블의 소켓과 연결.
- proc_pidpath로 이름·경로 메타데이터 조회. [S20][S21][S26]

현재 Apple 공개 구현에서 결합에 사용할 수 있는 필드 대응은 다음과 같다. [S22][S23]

| pcblist64 | libproc socket_fdinfo |
| --- | --- |
| xt_inpcb.xi_socket.xso_so | psi.soi_so |
| xt_inpcb.xi_socket.so_pcb | psi.soi_pcb |
| xt_inpcb.inp_gencnt | psi.soi_proto.pri_tcp.tcpsi_ini.insi_gencnt |

socket/PCB opaque token·generation과 family·주소·포트를 함께 검사한다. token이 0·누락·불일치이면 주소·포트만 보고 PID를 어림붙이지 않는다. 그 소켓은 미매핑으로 남긴다.

이 대응은 조사한 Apple 소스에 근거한 구현 선택이며 영구 보장되는 고수준 API 계약이라고 표현하지 않는다. 지원 SDK·OS·CPU에서 실제 결합 검증을 배포 조건으로 둔다. 토큰은 실행 안에서만 쓰고 공개 JSON·표·파일에 저장하지 않는다.

### 12.3 권한·경합

다른 사용자의 PID/FD 조회에는 권한 검사가 있으며 root에도 추가 정책이 적용될 수 있다. [S21]

- EPERM/EACCES: 관찰된 권한 제한.
- ESRCH: PID 조회가 더 이상 유효하지 않은 관찰.
- EBADF/ENOTSOCK: FD가 닫히거나 교체된 경합을 포함하는 조회 실패.
- 반환 길이가 구조체 크기보다 짧으면 성공 구조체로 해석하지 않음.
- C wrapper에서 호출 직전 errno를 초기화하고 반환값·직후 errno를 함께 캡처.
- 개별 FD 실패 때문에 정상 수집한 커널 endpoint를 제거하지 않음.
- 관계없는 PID의 FD 거부만으로 collection.complete를 false로 바꾸지 않음.
- 선택된 소켓의 PID를 연결하지 못하면 ownership_status로 표현.

### 12.4 IPv6·native 구조체·CGO

Darwin의 scoped IPv6 주소에서 embedded scope가 사용되는 경우 Apple netstat와 SDK 정의에 따라 scope를 분리하고 주소를 정규화한다. libproc의 interface 필드가 항상 채워져 있다고 가정하지 않는다. global IPv6 주소에 scope 추출 규칙을 무조건 적용하지 않는다. [S23][S24]

Darwin 전용 작은 C shim이 다음을 담당한다.

- SDK 시스템 헤더와 union·padding·sizeof 처리.
- sysctl/libproc 호출 및 errno 캡처.
- host-order 포트, IP 바이트 배열, scope, 내부 결합 토큰 추출.
- C 메모리의 명시적 해제.

Go 공통 계층에는 자체 정의한 단순 구조체를 전달한다. PRIVATE pcblist_n 구조체를 복사해 첫 구현의 범위를 확대하지 않는다.

macOS 지원 빌드는 CGO_ENABLED=1이며 macOS SDK·C compiler가 필요하다. 일반 Go 교차 컴파일만으로 cgo 대상 빌드가 해결되는 것은 아니다. [S25]

## 13. namespace·Docker·WSL

Windows·macOS에서 실행하면 각 호스트 OS에 관찰되는 소켓을 조회한다. Docker Desktop의 게시 포트가 호스트 전달 프로세스와 연결되면 그 PID를 출력하며 컨테이너 내부 PID로 치환하지 않는다. [S8]

Linux 컨테이너 안에서 실행하면 그 실행환경의 network namespace와 procfs 범위에서 조회한다. 자동으로 호스트 또는 다른 컨테이너의 namespace에 들어가지 않는다.

WSL 안에서 Linux 바이너리를 실행하면 해당 Linux 환경을 조회한다. Windows 바이너리로 실행하면 Windows 환경을 조회한다. 두 결과를 같은 전체 시스템 목록으로 취급하지 않는다. WSL 네트워킹 모드에 따라 호스트 관측 방식이 달라질 수 있다. [S9]

JSON runtime 필드와 README에 범위를 명시한다. Docker CLI·wsl·nsenter를 자동 호출하지 않는다.

## 14. 모듈 구조

| 예정 경로 | 책임 |
| --- | --- |
| portwho/go.mod | 독립 모듈·의존성 |
| portwho/main.go | 진입점·최종 종료 |
| portwho/cmd/root.go | 공통 CLI·인수·옵션 |
| portwho/internal/inspect/model.go | 원시 소켓·endpoint·owner·오류 모델 |
| portwho/internal/inspect/service.go | 수집·PID 연결·집계·종료 코드 |
| portwho/internal/platform/provider_windows.go | Windows API |
| portwho/internal/platform/provider_linux.go | Netlink·procfs |
| portwho/internal/platform/provider_darwin.go | Darwin Go adapter, cgo build constraint |
| portwho/internal/platform/native_darwin.c | 시스템 헤더 기반 C shim |
| portwho/internal/platform/native_darwin.h | Go/C 경계의 자체 타입 |
| portwho/internal/platform/provider_darwin_nocgo.go | 비지원 빌드의 명확한 오류 |
| portwho/internal/platform/provider_unsupported.go | 대상 밖 환경 처리 |
| portwho/internal/render/render.go | 표·JSON·PID |
| portwho/README.md | 지원 범위·설치·사용법·종료 코드 |

Go·Cobra로 공통 계층을 작성한다. Windows/Linux는 CGO_ENABLED=0 빌드를 제공하고, macOS만 최소 C shim을 사용한다. 시스템 API/라이브러리를 이용하며 별도 백그라운드 helper 실행 파일을 설치하지 않는다.

OS별 수집기·PID 연결기·메타데이터 조회기를 주입 가능한 경계로 둔다. 포트마다 전체 테이블이나 전체 PID 목록을 반복 수집하지 않는다. 공통 코드에서 OS별 예외로 출력·종료 의미를 달리하지 않는다.

## 15. 실행 순서

1. 도움말·버전 또는 일반 조회를 구분한다.
2. 옵션·인수 검증과 포트 정규화.
3. 플랫폼 수집기 선택 및 runtime 범위 설정.
4. 소켓 수집 시작 시각 기록.
5. 요청 family의 원시 TCP LISTEN 소켓 수집.
6. 수집 종료 시각 기록.
7. 선택 포트 필터.
8. 원시 소켓별 PID 연결, 확인된 가시성 제한 기록.
9. 원시 소켓별 매핑 상태를 유지한 채 endpoint·owners 집계.
10. 표·JSON 모드에서 고유 PID별 이미지 메타데이터 조회.
11. endpoint·owners 정렬, 종료 코드 판정.
12. 모드별 출력. PID 모드에서 핵심 오류이면 stdout을 쓰지 않음.
13. stdout/stderr 쓰기 실패 시 종료 코드 3.

PID 모드에서도 8번은 수행하며 10번만 생략한다. 원시 소켓을 endpoint로 먼저 합쳐 미매핑 소켓을 감추지 않는다.

## 16. 빌드·배포

| 대상 | 빌드 방식 | 실제 실행 검증 |
| --- | --- | --- |
| windows/amd64 | Go, CGO_ENABLED=0 | Windows 11 amd64 |
| linux/amd64 | Go, CGO_ENABLED=0 | Linux amd64 |
| linux/arm64 | Go, CGO_ENABLED=0 | Linux arm64 |
| darwin/amd64 | macOS SDK·C compiler, CGO_ENABLED=1 | Intel Mac |
| darwin/arm64 | macOS SDK·C compiler, CGO_ENABLED=1 | Apple Silicon Mac |

- OS·CPU별 배포 파일과 체크섬을 제공한다.
- 배포 바이너리 사용자는 Go·C compiler를 설치할 필요가 없다.
- macOS 바이너리는 기본 시스템 라이브러리에 연결하며 이를 완전한 정적 바이너리라고 설명하지 않는다.
- 소스에서 go install/build하는 macOS 사용자는 Go와 호환되는 Xcode Command Line Tools/SDK가 필요하다.
- Darwin CGO_ENABLED=0 빌드는 지원 바이너리로 배포하지 않는다. 해당 빌드의 조회는 backend_unavailable 및 3, 도움말·버전만 정상 동작할 수 있다.
- Linux/macOS의 실제 수집기를 비지원 stub으로 대체하여 릴리스 조건을 통과시킬 수 없다.
- 지원 CPU의 크로스 빌드만 성공한 상태를 해당 CPU 지원 검증 완료로 취급하지 않는다.
- README에 바이너리 설치와 소스 빌드 요건을 구분하여 적는다.

구현·배포 후 소스 설치 명령은 다음 형태이다.

```sh
go install github.com/w00j00ng/goproj/portwho@latest
```

이 문서는 설치 가능한 릴리스가 현재 존재한다고 주장하지 않는다.

## 17. 비기능 요구사항

- 원격 네트워크 연결·DNS 조회·포트 bind/connect를 수행하지 않는다.
- 프로세스 종료·시스템 설정 변경·자동 권한 상승을 하지 않는다.
- 결과를 외부 전송하거나 별도 로그·캐시 파일로 저장하지 않는다.
- 같은 명령 실행 안에서 PID별 메타데이터를 재사용하고 이후 실행에는 남기지 않는다.
- 모든 동적 버퍼의 길이·상한·오버플로를 검사한다.
- native handle·FD·C 할당 메모리를 성공·실패 경로 모두에서 해제한다.
- 큰 프로세스/FD 목록을 endpoint마다 중복 스캔하지 않는다.

성능은 구현 후 OS별로 측정한다. 초기 목표는 기준 개발 환경에서 리스너 1,000개 이하, 관찰 PID 500개 이하, 스캔 FD 10,000개 이하 조건의 일반 조회 p95 2초 이내이다. 이는 측정값·실시간 응답 보장이 아니다.

보고에는 OS·CPU·보안 정책·PID/FD/소켓 수·출력 모드를 기록한다. PID 모드는 이름·경로 조회를 생략하지만 Unix의 소켓→PID 연결 비용은 남는다. 공개 timeout 옵션은 첫 릴리스에 추가하지 않는다.

## 18. 사용자 관점 수용 기준

CLI 인수, 수집 결과, PID 연결 결과, 메타데이터 결과를 입력으로 하여 stdout·stderr·JSON 값·종료 코드를 관측한다. 소스 코드 문자열 포함 여부를 수용 테스트로 사용하지 않는다.

| ID | 입력·조건 | 기대 결과 | 코드 |
| --- | --- | --- | --- |
| A01 | 8080 LISTEN, 18080 LISTEN, 8080 ESTABLISHED | 8080 LISTEN만 포함 | 0 |
| A02 | 8080만 존재; 8080 9090 조회 | 8080 결과 | 0 |
| A03 | 요청 family 정상, 일치 소켓 없음 | 모드별 빈 결과 | 1 |
| A04 | --all, 여러 포트·상태 | 지원 범위 TCP LISTEN만 | 0 |
| A05 | 같은 PID의 IPv4·IPv6 | 별도 endpoint 유지 | 0 |
| A06 | --family ipv4 | IPv6 결과 제외; Windows/Linux는 IPv6 요청 생략 | 0/1 |
| A07 | IPv4만 존재, --family ipv6 정상 수집 | 빈 결과 | 1 |
| A08 | 옵션을 포트 앞·뒤에 배치 | 동일 의미 | 동일 |
| A09 | 08080 8080 | 정규화·중복 제거 | 0/1 |
| A10 | --pids, 동일 PID 여러 endpoint | 정렬된 고유 PID | 0 |
| A11 | --pids | 메타데이터 조회 생략, Unix PID 연결은 수행 | 0/1 |
| A12 | --json | JSON 객체 하나로 파싱 | 0/1 |
| A13 | --path, 접근 가능 | 이미지 경로 포함 | 0 |
| A14 | --path 없음 | path=null, PATH 열 없음 | 0 |
| A15 | 인수 없음 | 사용법 stderr, 조회 안 함 | 2 |
| A16 | 잘못된 숫자·범위·서비스 이름 | 각 입력 거부 | 2 |
| A17 | 상호 배타 옵션 조합 | 각 조합 거부 | 2 |
| A18 | 미정의 옵션·잘못된 family·값 누락 | 입력 오류 | 2 |
| A19 | --help/--version | OS 조회 없음 | 0 |
| A20 | PID 확인, 이름 접근 거부 | owner 유지, metadata unavailable | 0 |
| A21 | PID 연결 후 이미지 조회 실패 | endpoint·PID 유지 | 0 |
| A22 | Windows 이미지 API 오류 87 | os_error, PID 부재를 단정하지 않음 | 0 |
| A23 | IPv4 성공·IPv6 실패, 표 | 성공 결과와 진단 | 3 |
| A24 | 같은 상황, JSON | collection.complete=false | 3 |
| A25 | 같은 상황, --pids | stdout 0바이트 | 3 |
| A26 | 전체 실패 또는 실패+정상 빈 family | 완전한 빈 결과로 보고하지 않음 | 3 |
| A27 | IPv6 wildcard만 존재 | IPv4 행을 생성하지 않음 | 0 |
| A28 | 같은 IPv6 주소, scope 다름 | endpoint 분리 | 0 |
| A29 | 이미지 경로에 한글·공백 | 출력 보존 | 0 |
| A30 | stdout 또는 stderr writer 실패 | 실패 반환, 무한 재보고 없음 | 3 |
| A31 | 대상 밖 OS/CPU에서 정상 조회 | unsupported 진단 | 3 |
| A32 | 대상 밖 환경의 도움말·버전 | 정상 출력 | 0 |
| A33 | native 길이·개수·overflow 오류 | panic 없이 family 실패 | 3 |
| A34 | 수집 버퍼·재시도 상한 초과 | resource_limit | 3 |
| A35 | 소켓 존재, PID 연결 전부 실패 | owners=[], unresolved, endpoint 유지 | 3 |
| A36 | 위 상황 --pids | stdout 0바이트 | 3 |
| A37 | 동일 소켓을 두 PID가 공유, 둘 다 관찰 | owners에 둘 다, resolved | 0 |
| A38 | 한 PID가 동일 소켓의 FD 여러 개 보유 | owner 한 개 | 0 |
| A39 | 같은 endpoint의 서로 다른 소켓, 각각 PID 확인 | endpoint 하나, PID 합집합 | 0 |
| A40 | 위 두 소켓 중 하나만 PID 확인 | partial, 확인된 PID 유지 | 3 |
| A41 | 공유 소켓의 한 PID 확인, 추가 보유 PID 접근 거부 | resolved·limitation, 전체 보유자 완전 열거로 표현하지 않음 | 0 |
| A42 | hidepid로 PID 디렉터리가 숨겨짐 | 알려진 제한 기록, 미매핑 소켓 유지 | 상태에 따름 |
| A43 | Linux 현재 net namespace 밖에만 리스너 존재 | 외부 namespace 결과를 섞지 않음 | 1 |
| A44 | Linux procfs로 선택 소켓의 PID를 못 찾음 | 소켓 결과 유지, unresolved | 3 |
| A45 | Linux Netlink 잘림·중단·오류 | 제한된 재수집, 정상 빈 결과로 변환하지 않음 | 실패 지속 시 3 |
| A46 | macOS 커널 테이블 성공, 무관 PID의 FD 거부 | collection.complete 유지, limitation 기록 | 선택 소켓 resolved이면 0 |
| A47 | macOS token/generation 불일치 | 주소·포트로 추정 연결 금지, 미매핑 유지 | 3 |
| A48 | macOS 테이블 자체 오류, libproc는 일부 성공 | 전체 테이블 실패를 감추지 않음 | 3 |
| A49 | Darwin CGO_ENABLED=0 빌드의 조회 | backend_unavailable, 지원 바이너리로 배포 금지 | 3 |
| A50 | 모든 필수 OS/CPU에서 같은 공통 fixture | 옵션·JSON 의미·정렬·종료 코드 일치 | 동일 |

0/1 사례는 수집·최소 PID 연결·출력이 성공한 조건을 따른다. 추가 실패가 있으면 종료 코드 판정 순서를 적용한다.

## 19. 검증 방식과 완료 기준

### 19.1 OS 독립 검증

테이블 테스트로 실제 CLI 실행 함수를 호출하고 입력·기대 stdout/stderr·파싱한 JSON·종료 코드를 검사한다.

- 인수 정규화와 상호 배타 옵션.
- 원시 소켓 유지 → PID 연결 → endpoint 집계 순서.
- 복수 owner·SO_REUSEPORT·미매핑 소켓.
- PID 연결 실패와 이미지 메타데이터 실패의 구분.
- limitations가 있는 resolved와 실제 partial의 구분.
- JSON 타입·null·빈 배열·정렬·runtime 정보.
- 출력 실패·미지원 빌드.
- 잘린 native 데이터·Netlink 오류·버퍼 증가 경합.

### 19.2 네이티브 통합 검증

각 필수 OS/CPU에서 자식 테스트 프로세스가 `127.0.0.1:0`에 리슨하고 배정 포트·PID를 부모에게 알린다. 그 포트로 실제 portwho 바이너리를 실행한다.

- 주소·포트·PID·실행 이미지 조회.
- IPv6 loopback에서도 같은 검증.
- --pids의 실제 PID 확인.
- --all에서 자식 endpoint 포함 확인.
- 조회 후 리스너와 자식 생존 확인.
- 일반 사용자 권한의 자기 프로세스 조회.
- 권한 제한에서 endpoint 보존·미확인 상태 확인.

Linux에는 공유 FD·복수 PID·SO_REUSEPORT·namespace·procfs 제한 사례를 추가한다. macOS에는 pcblist64↔libproc token/generation 결합, Intel/Apple Silicon 구조체, link-local scope, 다른 UID의 접근 제한을 추가한다.

빈 결과는 임의의 고정 포트가 비어 있다고 가정하지 않고 주입 fixture로 검사한다. --all은 테스트 소켓 포함을 검증하며 전체 시스템 목록의 정확한 고정값을 기대하지 않는다.

Windows/Linux/macOS 네이티브 실행 환경은 릴리스 검증에서 IPv4·IPv6를 제공해야 한다. 교차 컴파일이나 일부 로컬 테스트 skip으로 해당 OS 지원 완료를 대체하지 않는다.

### 19.3 첫 릴리스 완료 기준

1. 다섯 필수 OS/CPU 조합의 실제 수집기·배포 바이너리 존재.
2. 공통 CLI·표·JSON schema 2·PID·종료 코드 계약 구현.
3. 50개 수용 시나리오의 관련 사례 검증.
4. 각 OS에서 일반 사용자 자기 프로세스의 실제 IPv4·IPv6 조회 통과.
5. Windows PID 보고, Linux inode 연결, macOS opaque token 연결의 실제 검증.
6. 미매핑·권한 거부·부분 수집 실패에서 관측 결과를 보존.
7. 각 모듈 테스트·정적 검사·빌드 및 native 자원 해제 확인.
8. OS별 성능 측정 결과·기준 환경 기록.
9. 문서의 runtime 범위·설치 요건·오류·출력이 실제 동작과 일치.
10. Linux·macOS를 후속 작업으로 남긴 채 완료 선언하지 않음.

## 20. 후속 범위

첫 릴리스 이후의 후보는 Windows arm64, 추가 Linux 배포판 검증 확대, UDP, watch, 프로세스명 필터, macOS 비BSD 네트워킹, 컨테이너/namespace 상세 추적이다.

Linux와 macOS의 기본 지원은 후속 후보가 아니다. 이 문서의 필수 범위이다.

## 21. 근거 자료

자료는 OS/API 동작의 근거이다. CLI·스키마·지원 기준·재시도·메모리·성능 목표는 이 문서에서 정한 설계 선택이다. Apple main 브랜치에서 확인한 내부 구현의 필드 대응은 지원 SDK/OS 실제 검증으로 확인하고, 구현 시 참조한 commit을 개발 기록에 남긴다.

- [S1] Microsoft, [GetExtendedTcpTable](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable).
- [S2] Microsoft, [MIB_TCPROW_OWNER_PID](https://learn.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcprow_owner_pid).
- [S3] Microsoft, [MIB_TCP6ROW_OWNER_PID](https://learn.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcp6row_owner_pid).
- [S4] Microsoft, [QueryFullProcessImageNameW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-queryfullprocessimagenamew).
- [S5] Microsoft, [OpenProcess](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-openprocess).
- [S6] Microsoft, [Dual-Stack Sockets](https://learn.microsoft.com/en-us/windows/win32/winsock/dual-stack-sockets).
- [S7] Microsoft, [bind](https://learn.microsoft.com/en-us/windows/win32/api/winsock/nf-winsock-bind).
- [S8] Docker, [Networking on Docker Desktop](https://docs.docker.com/desktop/features/networking/).
- [S9] Microsoft, [Accessing network applications with WSL](https://learn.microsoft.com/en-us/windows/wsl/networking).
- [S10] Microsoft, [MIB_TCP6TABLE_OWNER_PID](https://learn.microsoft.com/en-us/windows/win32/api/tcpmib/ns-tcpmib-mib_tcp6table_owner_pid).
- [S11] Go, [golang.org/x/sys/windows](https://pkg.go.dev/golang.org/x/sys/windows).
- [S12] Linux man-pages, [sock_diag(7)](https://man7.org/linux/man-pages/man7/sock_diag.7.html).
- [S13] Linux man-pages, [proc_pid_fd(5)](https://man7.org/linux/man-pages/man5/proc_pid_fd.5.html).
- [S14] Linux Kernel, [The /proc Filesystem](https://docs.kernel.org/filesystems/proc.html).
- [S15] Linux Kernel, [inet_diag UAPI](https://github.com/torvalds/linux/blob/master/include/uapi/linux/inet_diag.h).
- [S16] Linux Kernel, [proc/net/tcp and tcp6](https://docs.kernel.org/networking/proc_net_tcp.html).
- [S17] Linux man-pages, [proc_pid_exe(5)](https://man7.org/linux/man-pages/man5/proc_pid_exe.5.html).
- [S18] Linux man-pages, [netlink(7)](https://man7.org/linux/man-pages/man7/netlink.7.html).
- [S19] Apple XNU, [tcp_subr.c](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/netinet/tcp_subr.c).
- [S20] Apple XNU, [proc_info.h](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/proc_info.h).
- [S21] Apple XNU, [proc_info.c](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/proc_info.c).
- [S22] Apple XNU, [uipc_socket2.c](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/uipc_socket2.c).
- [S23] Apple XNU, [socket_info.c](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/socket_info.c).
- [S24] Apple network_cmds, [netstat inet6.c](https://github.com/apple-oss-distributions/network_cmds/blob/main/netstat.tproj/inet6.c).
- [S25] Go, [cgo](https://pkg.go.dev/cmd/cgo).
- [S26] Apple XNU, [libproc.h](https://github.com/apple-oss-distributions/xnu/blob/main/libsyscall/wrappers/libproc/libproc.h).
- [S27] Apple XNU, [in_pcblist.c](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/netinet/in_pcblist.c).
- [S28] Linux man-pages, [proc_pid_comm(5)](https://man7.org/linux/man-pages/man5/proc_pid_comm.5.html).
- [S29] Linux man-pages, [fork(2)](https://man7.org/linux/man-pages/man2/fork.2.html).
- [S30] Linux man-pages, [socket(7)](https://man7.org/linux/man-pages/man7/socket.7.html).
- [S31] Linux man-pages, [proc_pid_net(5)](https://man7.org/linux/man-pages/man5/proc_pid_net.5.html).
- [S32] Linux man-pages, [network_namespaces(7)](https://man7.org/linux/man-pages/man7/network_namespaces.7.html).
