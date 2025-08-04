#include <windows.h>
#include <wchar.h>
#include <shellapi.h>
#include <stdlib.h>

#ifndef WM_CLIPBOARDUPDATE
#define WM_CLIPBOARDUPDATE 0x031D
#endif

#define POPUP_LIFETIME_MS   1000
#define WINDOW_CLASS_NAME   L"ClipboardPopupWindow"
#define TRAY_ICON_UID       1
#define WM_TRAYICON         (WM_USER + 1)
#define TRANSPARENCY_LEVEL  50

WCHAR *lastText = NULL;
HWND   hPopupWnd = NULL;
WCHAR  currentMessage[8192] = L"";
HFONT  hFont = NULL;
NOTIFYICONDATA nid = { 0 };

void CleanupTrayIcon() {
    Shell_NotifyIcon(NIM_DELETE, &nid);
}

void ShowPopup(HWND hOwner, const WCHAR* text) {
    // 이전 팝업 제거
    if (hPopupWnd) {
        DestroyWindow(hPopupWnd);
    }

    // 메시지 복사
    wcsncpy(currentMessage, text, _countof(currentMessage)-1);
    currentMessage[_countof(currentMessage)-1] = L'\0';

    // 폰트 생성 (한 번만)
    if (!hFont) {
        hFont = CreateFontW(
            -16, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE,
            DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS,
            CLEARTYPE_QUALITY, DEFAULT_PITCH|FF_DONTCARE, L"Segoe UI"
        );
    }

    // 텍스트 크기 측정
    HDC hdc = GetDC(NULL);
    SelectObject(hdc, hFont);
    RECT rc = {0,0,400,0};
    DrawTextW(hdc, currentMessage, -1, &rc, DT_CALCRECT|DT_WORDBREAK);
    int w = min(rc.right + 20, 400);
    int h = min(rc.bottom + 20, 300);
    ReleaseDC(NULL, hdc);

    // 화면 우하단 위치 계산
    int x = GetSystemMetrics(SM_CXSCREEN) - w - 20;
    int y = GetSystemMetrics(SM_CYSCREEN) - h - 60;

    // 팝업 윈도우 생성
    hPopupWnd = CreateWindowExW(
        WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE|WS_EX_LAYERED,
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

void AddTrayIcon(HWND hwnd) {
    nid.cbSize           = sizeof(nid);
    nid.hWnd             = hwnd;
    nid.uID              = TRAY_ICON_UID;
    nid.uFlags           = NIF_ICON|NIF_MESSAGE|NIF_TIP;
    nid.uCallbackMessage = WM_TRAYICON;
    nid.hIcon            = LoadIcon(NULL, IDI_INFORMATION);
    wcscpy(nid.szTip, L"Clipflash");
    Shell_NotifyIcon(NIM_ADD, &nid);
}

LRESULT CALLBACK WndProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
    switch (msg) {
    case WM_CLIPBOARDUPDATE: {
        // 클립보드에 텍스트가 복사되었을 때
        if (OpenClipboard(NULL)) {
            HANDLE hData = GetClipboardData(CF_UNICODETEXT);
            if (hData) {
                WCHAR *clipText = GlobalLock(hData);
                if (clipText) {
                    if (!lastText || wcscmp(clipText, lastText) != 0) {
                        if (lastText) free(lastText);
                        size_t len = (wcslen(clipText) + 1) * sizeof(WCHAR);
                        lastText = malloc(len);
                        memcpy(lastText, clipText, len);
                        ShowPopup(hwnd, clipText);
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
            AppendMenuW(hMenu, MF_STRING, 1, L"종료");
            POINT pt; GetCursorPos(&pt);
            SetForegroundWindow(hwnd);
            int cmd = TrackPopupMenu(
                hMenu,
                TPM_RETURNCMD|TPM_NONOTIFY,
                pt.x, pt.y,
                0, hwnd, NULL
            );
            DestroyMenu(hMenu);
            if (cmd == 1) {
                CleanupTrayIcon();
                PostQuitMessage(0);
            }
        }
        break;
    case WM_PAINT: {
        // 팝업 텍스트 그리기
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        SetBkMode(hdc, TRANSPARENT);
        SetTextColor(hdc, RGB(30,30,30));
        SelectObject(hdc, hFont);
        RECT rc; GetClientRect(hwnd, &rc);
        InflateRect(&rc, -10, -10);
        DrawTextW(hdc, currentMessage, -1, &rc, DT_WORDBREAK|DT_LEFT|DT_TOP);
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
    PWSTR    pCmdLine,
    int      nCmdShow
) {
    // 윈도우 클래스 등록
    WNDCLASSW wc = {
        .lpfnWndProc   = WndProc,
        .hInstance     = hInst,
        .lpszClassName = WINDOW_CLASS_NAME,
        .hbrBackground = CreateSolidBrush(RGB(255,255,255)),
        .hCursor       = LoadCursor(NULL, IDC_ARROW)
    };
    RegisterClassW(&wc);

    // 히든 윈도우 + 트레이 아이콘
    HWND hWnd = CreateWindowExW(
        0, WINDOW_CLASS_NAME, L"Main",
        0, 0,0,0,0,
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