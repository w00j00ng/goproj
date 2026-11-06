"""Report confirmed scheduled-main findings in one open issue."""

from datetime import datetime, timedelta, timezone
import json
import os
from urllib.request import Request, urlopen


ISSUE_MARKER = "<!-- dependency-security-issue:v1 -->"
FINDING_STEP = "Fail for reachable vulnerabilities"
BOT = "github-actions[bot]"


def main():
    if (os.environ["GITHUB_EVENT_NAME"] != "schedule"
            or os.environ["GITHUB_REF"] != "refs/heads/main"):
        return

    root = f"{os.environ['GITHUB_API_URL']}/repos/{os.environ['GITHUB_REPOSITORY']}"

    def api(method, path, payload=None):
        request = Request(
            f"{root}/{path}", method=method,
            data=json.dumps(payload).encode() if payload is not None else None,
            headers={
                "Accept": "application/vnd.github+json",
                "Authorization": f"Bearer {os.environ['GH_TOKEN']}",
                "X-GitHub-Api-Version": "2026-03-10",
                "Content-Type": "application/json",
            },
        )
        with urlopen(request, timeout=30) as response:
            return json.load(response)

    def pages(path, key=None):
        separator = "&" if "?" in path else "?"
        page = 1
        while True:
            result = api("GET", f"{path}{separator}per_page=100&page={page}")
            items = result[key] if key else result
            yield from items
            if len(items) < 100:
                return
            page += 1

    run_id = os.environ["GITHUB_RUN_ID"]
    attempt = os.environ["GITHUB_RUN_ATTEMPT"]
    run = api("GET", f"actions/runs/{run_id}")
    if run["event"] != "schedule" or run["head_branch"] != "main":
        return

    jobs = [
        job for job in pages(f"actions/runs/{run_id}/attempts/{attempt}/jobs", "jobs")
        if job["name"].startswith("govulncheck (") and job["status"] == "completed"
    ]
    findings = [
        job for job in jobs
        if job["conclusion"] == "failure" and any(
            step["name"] == FINDING_STEP and step["conclusion"] == "failure"
            for step in job.get("steps", [])
        )
    ]
    if not findings:
        return

    run_url = (f"{os.environ['GITHUB_SERVER_URL']}/{os.environ['GITHUB_REPOSITORY']}"
               f"/actions/runs/{run_id}/attempts/{attempt}")
    scanned_at = datetime.fromisoformat(run["created_at"].replace("Z", "+00:00"))
    scanned_at = scanned_at.astimezone(timezone(timedelta(hours=9)))
    body = (
        f"{ISSUE_MARKER}\n"
        "main 정기 의존성 검사에서 호출 가능한 보안 취약점이 발견되었습니다.\n\n"
        f"- 검사일: {scanned_at:%Y-%m-%d %H:%M} KST\n"
        f"- 대상 커밋: `{run['head_sha']}`\n"
        f"- 실행: [검사 로그]({run_url})\n\n"
        "| 취약점 발견 모듈 | 상세 결과 |\n| --- | --- |\n"
    )
    for job in sorted(findings, key=lambda item: item["name"]):
        body += f"| {job['name']} | [취약점 및 호출 경로]({job['html_url']}) |\n"
    errors = [job["name"] for job in jobs if job["conclusion"] != "success" and job not in findings]
    if errors:
        body += "\n다음 모듈은 검사 오류로 판정하지 못했습니다: " + ", ".join(errors) + ".\n"
    body += (
        "\n검사 범위: Linux, CGO_ENABLED=0, 테스트 포함, govulncheck v1.9.0.\n"
        "본문은 최근 발견 결과로 갱신하며 알림은 별도 댓글로 남깁니다. "
        "조치 완료 후 이슈를 닫아 주세요. 이후 발견 시 새 이슈를 생성합니다.\n"
    )

    issue = next((
        item for item in pages("issues?state=open&creator=github-actions%5Bbot%5D")
        if "pull_request" not in item and item["user"]["login"] == BOT
        and ISSUE_MARKER in (item.get("body") or "")
    ), None)
    if issue:
        api("PATCH", f"issues/{issue['number']}", {"body": body})
    else:
        issue = api("POST", "issues", {"title": "의존성 보안 취약점 정기 검사", "body": body})

    comment_path = f"issues/{issue['number']}/comments"
    day_marker = f"<!-- dependency-security-alert:{scanned_at:%Y-%m-%d} -->"
    if not any(
        item["user"]["login"] == BOT and day_marker in (item.get("body") or "")
        for item in pages(comment_path)
    ):
        api("POST", comment_path, {"body": (
            f"{day_marker}\n@w00j00ng {scanned_at:%Y-%m-%d} KST 정기 검사에서 "
            f"보안 취약점이 발견되었습니다. [검사 결과]({run_url})와 갱신된 본문을 확인해 주세요."
        )})


if __name__ == "__main__":
    main()
