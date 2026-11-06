import json
import os
from pathlib import Path
import subprocess
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse


SCRIPT = Path(__file__).with_name("security_issue.py")
MARKER = "<!-- dependency-security-issue:v1 -->"
BOT = {"login": "github-actions[bot]"}


class SecurityIssueTest(unittest.TestCase):
    def setUp(self):
        self.issues = []
        self.comments = []
        self.requests = []
        self.fail_method = None
        self.fail_path = None
        self.run = {
            "event": "schedule",
            "head_branch": "main",
            "head_sha": "a" * 40,
            "created_at": "2026-11-06T22:23:00Z",
        }
        self.jobs = [self.job("bcrypter", "failure")]
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def handle_request(self):
                url = urlparse(self.path)
                path = url.path.removeprefix("/repos/w00j00ng/goproj/")
                query = parse_qs(url.query)
                payload = None
                if length := int(self.headers.get("Content-Length", "0")):
                    payload = json.loads(self.rfile.read(length))
                owner.requests.append((self.command, path, payload))
                if self.command == owner.fail_method and (
                    owner.fail_path is None or path == owner.fail_path
                ):
                    self.send_error(503)
                    return

                def page(items):
                    start = (int(query.get("page", ["1"])[0]) - 1) * 100
                    return items[start:start + 100]

                if self.command == "GET" and path == "actions/runs/123":
                    result = owner.run
                elif self.command == "GET" and path == "actions/runs/123/attempts/2/jobs":
                    result = {"jobs": page(owner.jobs)}
                elif self.command == "GET" and path == "issues":
                    result = page([
                        issue for issue in owner.issues
                        if query.get("state") != ["open"] or issue.get("state", "open") == "open"
                    ])
                elif self.command == "GET" and path.endswith("/comments"):
                    number = int(path.split("/")[1])
                    result = page([
                        item for item in owner.comments if item.get("issue_number", 42) == number
                    ])
                elif self.command == "POST" and path == "issues":
                    number = max([issue.get("number", 0) for issue in owner.issues] + [41]) + 1
                    result = {"number": number, "state": "open", "user": BOT, **payload}
                    owner.issues.append(result)
                elif self.command == "PATCH" and path.startswith("issues/"):
                    number = int(path.split("/")[1])
                    result = next(issue for issue in owner.issues if issue.get("number") == number)
                    result.update(payload)
                elif self.command == "POST" and path.endswith("/comments"):
                    result = {"user": BOT, "issue_number": int(path.split("/")[1]), **payload}
                    owner.comments.append(result)
                else:
                    self.send_error(404)
                    return
                body = json.dumps(result).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            do_GET = do_POST = do_PATCH = handle_request

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)

    @staticmethod
    def job(module, finding, conclusion="failure"):
        return {
            "name": f"govulncheck ({module})",
            "status": "completed",
            "conclusion": conclusion,
            "html_url": f"https://github.com/w00j00ng/goproj/actions/runs/123/job/{module}",
            "steps": [{"name": "Fail for reachable vulnerabilities", "conclusion": finding}],
        }

    def invoke(self, **overrides):
        env = {
            **os.environ,
            "GITHUB_EVENT_NAME": "schedule",
            "GITHUB_REF": "refs/heads/main",
            "GITHUB_API_URL": f"http://127.0.0.1:{self.server.server_port}",
            "GITHUB_SERVER_URL": "https://github.com",
            "GITHUB_REPOSITORY": "w00j00ng/goproj",
            "GITHUB_RUN_ID": "123",
            "GITHUB_RUN_ATTEMPT": "2",
            "GH_TOKEN": "test-token",
            "PYTHONUTF8": "1",
            "NO_PROXY": "127.0.0.1,localhost",
            **overrides,
        }
        return subprocess.run(
            [sys.executable, "-B", str(SCRIPT)], env=env,
            capture_output=True, text=True, encoding="utf-8", timeout=10,
        )

    def test_scheduled_finding_creates_one_issue_and_one_mention_comment(self):
        # 요청: main 정기 검사에서 취약점 발견. 기대: 모듈·실행 링크를 담은 이슈 1건과 멘션 댓글 1건.
        self.jobs.append(self.job("scrypter", "skipped"))
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.issues), 1)
        self.assertIn(MARKER, self.issues[0]["body"])
        self.assertIn("bcrypter", self.issues[0]["body"])
        self.assertIn("/job/bcrypter", self.issues[0]["body"])
        self.assertIn("a" * 40, self.issues[0]["body"])
        self.assertIn("scrypter", self.issues[0]["body"])
        self.assertEqual(len(self.comments), 1)
        self.assertIn("@w00j00ng", self.comments[0]["body"])
        self.assertIn("2026-11-07", self.comments[0]["body"])

    def test_same_day_updates_body_and_next_day_comments_on_same_issue(self):
        # 요청: 같은 날 재실행 후 다음 날 재발견. 기대: 본문은 최신화하고 댓글만 날짜별 1건씩 추가.
        self.assertEqual(self.invoke().returncode, 0)
        self.jobs = [self.job("helloworld", "failure")]
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(len(self.issues), 1)
        self.assertIn("helloworld", self.issues[0]["body"])
        self.assertNotIn("bcrypter", self.issues[0]["body"])
        self.assertEqual(len(self.comments), 1)
        self.run["created_at"] = "2026-11-07T22:23:00Z"
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(len(self.issues), 1)
        self.assertEqual(len(self.comments), 2)

    def test_closed_issue_is_preserved_and_a_new_issue_is_created(self):
        # 요청: 기존 이슈를 닫은 후 다시 발견. 기대: 닫힌 이슈를 보존하고 새 이슈를 생성.
        self.issues = [{"number": 42, "state": "closed", "body": MARKER, "user": BOT}]
        self.comments = [{"body": "<!-- dependency-security-alert:2026-11-07 -->", "user": BOT}]
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.issues), 2)
        self.assertEqual(self.issues[0]["state"], "closed")
        self.assertEqual(self.issues[0]["body"], MARKER)
        self.assertEqual(self.issues[1]["number"], 43)
        self.assertEqual(self.issues[1]["state"], "open")
        self.assertEqual(len(self.comments), 2)
        self.assertEqual(self.comments[-1]["issue_number"], 43)

    def test_clean_scan_and_execution_errors_do_not_change_issues(self):
        # 요청: 정상·분석 오류·취소된 검사. 기대: 이슈/댓글 변경 없음.
        for conclusion in ("success", "failure", "cancelled"):
            with self.subTest(conclusion=conclusion):
                self.jobs = [self.job("bcrypter", "skipped", conclusion)]
                result = self.invoke()
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(self.issues, [])
                self.assertEqual(self.comments, [])
        self.assertTrue(all(method == "GET" for method, _, _ in self.requests))

    def test_other_events_and_branches_do_not_access_api(self):
        # 요청: PR·수동·push 또는 main 이외 브랜치. 기대: 조회/쓰기 모두 생략.
        for overrides in (
            {"GITHUB_EVENT_NAME": "pull_request"},
            {"GITHUB_EVENT_NAME": "workflow_dispatch"},
            {"GITHUB_EVENT_NAME": "push"},
            {"GITHUB_REF": "refs/heads/feature"},
        ):
            with self.subTest(overrides=overrides):
                result = self.invoke(**overrides)
                self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.requests, [])

    def test_pagination_does_not_duplicate_existing_issue_or_daily_comment(self):
        # 요청: 기존 이슈·당일 댓글이 두 번째 페이지에 위치. 기대: 모두 재사용.
        self.issues = [{"body": "other", "user": BOT} for _ in range(100)] + [
            {"number": 42, "state": "open", "body": MARKER, "user": BOT},
        ]
        self.comments = [{"body": "other", "user": BOT} for _ in range(100)] + [
            {"body": "<!-- dependency-security-alert:2026-11-07 -->", "user": BOT},
        ]
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.issues), 101)
        self.assertEqual(len(self.comments), 101)

    def test_run_metadata_must_also_be_scheduled_main(self):
        # 요청: 환경변수와 실제 실행 정보가 다름. 기대: 이슈/댓글 쓰기 없음.
        for field, value in (("event", "workflow_dispatch"), ("head_branch", "feature")):
            with self.subTest(field=field):
                original = self.run[field]
                self.run[field] = value
                result = self.invoke()
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(self.issues, [])
                self.assertEqual(self.comments, [])
                self.run[field] = original

    def test_api_failure_fails_job_without_sending_notification(self):
        # 요청: 이슈 본문 갱신 API 실패. 기대: 실행 실패, 알림 댓글은 발송하지 않음.
        self.issues = [{"number": 42, "state": "open", "body": MARKER, "user": BOT}]
        self.fail_method = "PATCH"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("HTTP Error 503", result.stderr)
        self.assertEqual(self.comments, [])
        self.assertNotIn("test-token", result.stdout + result.stderr)

    def test_comment_failure_can_be_retried_without_creating_another_issue(self):
        # 요청: 이슈 생성 후 알림 댓글 API 실패, 이후 재실행. 기대: 같은 이슈에 댓글 1건만 기록.
        self.fail_method = "POST"
        self.fail_path = "issues/42/comments"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("HTTP Error 503", result.stderr)
        self.assertEqual(len(self.issues), 1)
        self.assertEqual(self.comments, [])
        self.fail_method = None
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.issues), 1)
        self.assertEqual(len(self.comments), 1)


if __name__ == "__main__":
    unittest.main()
