#include <windows.h>
#include <wchar.h>
#include <shellapi.h>
#include <stdlib.h>
#include <math.h>

#ifndef WM_CLIPBOARDUPDATE
#define WM_CLIPBOARDUPDATE 0x031D
#endif

#define POPUP_LIFETIME_MS    1000
#define WINDOW_CLASS_NAME    L"ClipboardPopupWindow"
#define TRAY_ICON_UID        1
#define WM_TRAYICON          (WM_USER + 1)
#define TRANSPARENCY_LEVEL   50
#define ID_TRAY_EXIT         1
#define ID_TRAY_TOGGLE_PAUSE 2

// 복사된 텍스트 최대 처리 길이 (문자 수)
#define MAX_CLIP_TEXT_LENGTH 1024 * 1024 * 4

WCHAR *lastText = NULL;
HWND   hPopupWnd = NULL;
WCHAR  currentMessage[8192] = L"";
HFONT  hFont = NULL;
NOTIFYICONDATA nid = {0};
static BOOL paused = FALSE;  // 일시정지 상태 플래그

// 민감 정보 감지: URL 예외, 길이 6~64, 공백/제어문자 없음,
// 숫자·영문·특수문자 중 2가지 이상 조합, per-char 엔트로피 ≥ 3.0bits
static BOOL IsSensitive(const WCHAR *text)
{
    size_t len = wcslen(text);
    if (len < 6 || len > 64) {
        return FALSE;
    }

    // 1) URL 예외 처리
    if (wcsstr(text, L"://") != NULL || wcsstr(text, L"www.") != NULL) {
        return FALSE;
    }

    // 2) 공백·제어문자 배제
    for (size_t i = 0; i < len; i++) {
        if (iswspace(text[i]) || iswcntrl(text[i])) {
            return FALSE;
        }
    }

    // 3) 문자 카테고리 체크 (숫자, 영문, 특수문자)
    BOOL hasDigit = FALSE, hasAlpha = FALSE, hasSpecial = FALSE;
    for (size_t i = 0; i < len; i++) {
        wchar_t c = text[i];
        if (iswdigit(c)) {
            hasDigit = TRUE;
        } else if (iswalpha(c)) {
             hasAlpha = TRUE;
        } else {
            hasSpecial = TRUE;
        }
    }
    int categories = hasDigit + hasAlpha + hasSpecial;
    if (categories < 2) {
        return FALSE;
    }

    // 4) 샤논 엔트로피 계산 (ASCII 영역만)
    double freq[128] = {0};
    for (size_t i = 0; i < len; i++) {
        wchar_t c = text[i];
        if (c < 128) {
             freq[(int)c] += 1.0;
        }
    }
    double entropy = 0.0;
    for (int i = 0; i < 128; i++) {
        if (freq[i] > 0) {
            double p = freq[i] / (double)len;
            entropy -= p * (log(p) / log(2.0));
        }
    }
    // 평균 비트(entropy per char)
    double bitsPerChar = entropy;

    return bitsPerChar >= 3.0;
}

void CleanupTrayIcon(void)
{
    Shell_NotifyIcon(NIM_DELETE, &nid);
}

void ShowPopup(HWND hOwner, const WCHAR *text)
{
    // 이전 팝업 제거
    if (hPopupWnd) {
        DestroyWindow(hPopupWnd);
    }

    // 메시지 복사
    wcsncpy(currentMessage, text, _countof(currentMessage) - 1);
    currentMessage[_countof(currentMessage) - 1] = L'\0';

    // 폰트 생성 (한 번만)
    if (!hFont) {
        hFont = CreateFontW(
            -16, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE,
            DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS,
            CLEARTYPE_QUALITY, DEFAULT_PITCH | FF_DONTCARE, L"Segoe UI"
        );
    }

    // 텍스트 크기 측정
    HDC hdc = GetDC(NULL);
    SelectObject(hdc, hFont);
    RECT rc = {0, 0, 400, 0};
    DrawTextW(hdc, currentMessage, -1, &rc, DT_CALCRECT | DT_WORDBREAK);
    int w = min(rc.right + 20, 400);
    int h = min(rc.bottom + 20, 300);
    ReleaseDC(NULL, hdc);

    // ShowPopup 내 위치 계산 부분 수정
    POINT pt;
    GetCursorPos(&pt);
    HMONITOR hMon = MonitorFromPoint(pt, MONITOR_DEFAULTTONEAREST);
    MONITORINFO mi = {sizeof(mi)};
    GetMonitorInfoW(hMon, &mi);
    RECT work = mi.rcWork;

    int x = pt.x + 10;
    int y = pt.y + 20;
    // 우측/하단 경계
    if (x + w > work.right) {
        x = work.right - w - 5;
    }
    if (y + h > work.bottom) {
        y = work.bottom - h - 5;
    }
    // 좌측/상단 경계
    if (x < work.left) {
        x = work.left + 5;
    }
    if (y < work.top) {
        y = work.top + 5;
    }

    // 팝업 윈도우 생성
    hPopupWnd = CreateWindowExW(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE | WS_EX_LAYERED,
        WINDOW_CLASS_NAME, L"", WS_POPUP,
        x, y, w, h, hOwner, NULL, GetModuleHandle(NULL), NULL
    );
    SetLayeredWindowAttributes(
        hPopupWnd, 0,
        (255 * TRANSPARENCY_LEVEL) / 100,
        LWA_ALPHA
    );
    ShowWindow(hPopupWnd, SW_SHOWNOACTIVATE);
    SetTimer(hPopupWnd, 1, POPUP_LIFETIME_MS, NULL);
    UpdateWindow(hPopupWnd);
}

void AddTrayIcon(HWND hwnd)
{
    // 커스텀 아이콘 로드 시도
    HICON hIcon = (HICON)LoadImageW(
        NULL,  // 파일 시스템에서 직접 로드
        L"clipflash.ico",
        IMAGE_ICON,
        16, 16,  // 트레이 아이콘 크기
        LR_LOADFROMFILE
    );
    
    // 아이콘 로드 실패 시 기본 아이콘 사용
    if (!hIcon) {
        hIcon = LoadIcon(NULL, IDI_INFORMATION);
    }
    
    nid.cbSize = sizeof(nid);
    nid.hWnd = hwnd;
    nid.uID = TRAY_ICON_UID;
    nid.uFlags = NIF_ICON | NIF_MESSAGE | NIF_TIP;
    nid.uCallbackMessage = WM_TRAYICON;
    nid.hIcon = hIcon;
    wcscpy(nid.szTip, L"Clipflash");
    Shell_NotifyIcon(NIM_ADD, &nid);
}

LRESULT CALLBACK WndProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    switch (msg) {
    case WM_CLIPBOARDUPDATE: {
        // 클립보드에 텍스트가 복사되었을 때
        if (paused) {
            break;  // 일시정지 중이면 무시
        }
        if (OpenClipboard(NULL)) {
            HANDLE hData = GetClipboardData(CF_UNICODETEXT);
            if (hData) {
                WCHAR *clipText = GlobalLock(hData);
                if (clipText) {
                    // --- 새로 추가된 길이 검사 ---
                    size_t clipLen = wcslen(clipText);
                    if (clipLen > MAX_CLIP_TEXT_LENGTH) {
                        // 너무 긴 텍스트는 무시
                        GlobalUnlock(hData);
                        CloseClipboard();
                        break;
                    }
                    const WCHAR *displayText = clipText;
                    static const WCHAR placeholder[] = L"[보안 텍스트]";
                    if (IsSensitive(clipText)) {
                        displayText = placeholder;
                    }
                    if (!lastText || wcscmp(clipText, lastText) != 0) {
                        if (lastText) {
                            free(lastText);
                        }
                        size_t len = (wcslen(clipText) + 1) * sizeof(WCHAR);
                        lastText = malloc(len);
                        memcpy(lastText, clipText, len);
                        ShowPopup(hwnd, displayText);
                    }
                    GlobalUnlock(hData);
                }
            }
            CloseClipboard();
        }
        break;
    }
    case WM_TIMER:
        // 팝업 수명 종료 시
        DestroyWindow(hwnd);
        break;
    case WM_DESTROY:
        // 팝업 윈도우 파괴 후 정리
        KillTimer(hwnd, 1);
        hPopupWnd = NULL;
        break;
    case WM_TRAYICON:
        // 트레이 아이콘 우클릭 메뉴
        if (lParam == WM_RBUTTONUP) {
            HMENU hMenu = CreatePopupMenu();
            AppendMenuW(hMenu, MF_STRING, ID_TRAY_TOGGLE_PAUSE,
                       paused ? L"작업 재개" : L"일시정지");
            AppendMenuW(hMenu, MF_STRING, ID_TRAY_EXIT, L"종료");
            POINT pt;
            GetCursorPos(&pt);
            SetForegroundWindow(hwnd);
            int cmd = TrackPopupMenu(hMenu,
                TPM_RETURNCMD | TPM_NONOTIFY,
                pt.x, pt.y, 0, hwnd, NULL);
            DestroyMenu(hMenu);
            if (cmd == ID_TRAY_EXIT) {
                CleanupTrayIcon();
                PostQuitMessage(0);
            } else if (cmd == ID_TRAY_TOGGLE_PAUSE) {
                paused = !paused;
                ShowPopup(hwnd,
                    paused ? L"일시정지됨" : L"작업 재개됨");
            }
        }
        break;
    case WM_PAINT: {
        // 팝업 텍스트 그리기
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        SetBkMode(hdc, TRANSPARENT);
        SetTextColor(hdc, RGB(30, 30, 30));
        SelectObject(hdc, hFont);
        RECT rc;
        GetClientRect(hwnd, &rc);
        InflateRect(&rc, -10, -10);
        DrawTextW(hdc, currentMessage, -1, &rc, DT_WORDBREAK | DT_LEFT | DT_TOP);
        EndPaint(hwnd, &ps);
        break;
    }
    default:
        return DefWindowProcW(hwnd, msg, wParam, lParam);
    }
    return 0;
}

int WINAPI wWinMain(
    HINSTANCE hInst,
    HINSTANCE hPrevInstance,
    PWSTR pCmdLine,
    int nCmdShow
)
{
    // 커스텀 아이콘 로드
    HICON hAppIcon = (HICON)LoadImageW(
        NULL,  // 파일 시스템에서 직접 로드
        L"clipflash.ico",
        IMAGE_ICON,
        32, 32,  // 윈도우 아이콘 크기
        LR_LOADFROMFILE
    );
    
    // 아이콘 로드 실패 시 NULL 사용 (기본 아이콘)
    if (!hAppIcon) {
        hAppIcon = NULL;
    }
    
    // 윈도우 클래스 등록
    WNDCLASSW wc = {
        .lpfnWndProc = WndProc,
        .hInstance = hInst,
        .lpszClassName = WINDOW_CLASS_NAME,
        .hbrBackground = CreateSolidBrush(RGB(255, 255, 255)),
        .hCursor = LoadCursor(NULL, IDC_ARROW),
        .hIcon = hAppIcon  // 커스텀 아이콘 설정
    };
    RegisterClassW(&wc);

    // 히든 윈도우 + 트레이 아이콘
    HWND hWnd = CreateWindowExW(
        0, WINDOW_CLASS_NAME, L"Main",
        0, 0, 0, 0, 0,
        NULL, NULL, hInst, NULL
    );
    AddTrayIcon(hWnd);

    // 클립보드 업데이트 이벤트 수신 등록
    AddClipboardFormatListener(hWnd);

    // 메시지 루프 (이벤트 기반)
    MSG msg;
    while (GetMessage(&msg, NULL, 0, 0)) {
        TranslateMessage(&msg);
        DispatchMessage(&msg);
    }

    // 종료 전 정리
    RemoveClipboardFormatListener(hWnd);
    CleanupTrayIcon();
    free(lastText);
    return 0;
}