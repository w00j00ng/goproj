#include <windows.h>
#include <wchar.h>
#include <shellapi.h>
#include <stdlib.h>

#define POPUP_LIFETIME_MS   1000
#define WINDOW_CLASS_NAME   L"ClipboardPopupWindow"
#define TRAY_ICON_UID       1
#define WM_TRAYICON         (WM_USER + 1)
#define TRANSPARENCY_LEVEL  50
#define POLL_INTERVAL_MS    200

WCHAR *lastText = NULL;
HWND   hPopupWnd = NULL;
WCHAR  currentMessage[8192] = L"";
HFONT  hFont = NULL;
NOTIFYICONDATA nid = { 0 };

void CleanupTrayIcon() {
    Shell_NotifyIcon(NIM_DELETE, &nid);
}

LRESULT CALLBACK WndProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
    switch (msg) {
    case WM_TIMER:
        DestroyWindow(hwnd);
        break;
    case WM_DESTROY:
        KillTimer(hwnd, 1);
        hPopupWnd = NULL;
        break;
    case WM_TRAYICON:
        if (lParam == WM_RBUTTONUP) {
            HMENU hMenu = CreatePopupMenu();
            AppendMenu(hMenu, MF_STRING, 1, L"종료");
            POINT pt; GetCursorPos(&pt);
            SetForegroundWindow(hwnd);
            if (TrackPopupMenu(hMenu, TPM_RETURNCMD|TPM_NONOTIFY, pt.x, pt.y, 0, hwnd, NULL)==1) {
                CleanupTrayIcon();
                PostQuitMessage(0);
            }
        }
        break;
    case WM_PAINT: {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        SetBkMode(hdc, TRANSPARENT);
        SetTextColor(hdc, RGB(30,30,30));
        SelectObject(hdc, hFont);
        RECT rect; GetClientRect(hwnd, &rect);
        InflateRect(&rect, -10, -10);
        DrawTextW(hdc, currentMessage, -1, &rect, DT_WORDBREAK|DT_LEFT|DT_TOP);
        EndPaint(hwnd, &ps);
        break;
    }
    default:
        return DefWindowProcW(hwnd, msg, wParam, lParam);
    }
    return 0;
}

void ShowPopup(HWND hOwner, const WCHAR* text) {
    if (hPopupWnd) {
        DestroyWindow(hPopupWnd);
    }

    // 메시지 복사 (max 8191 chars)
    wcsncpy(currentMessage, text, _countof(currentMessage)-1);
    currentMessage[_countof(currentMessage)-1] = L'\0';

    // 폰트 준비
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
    RECT textRect = {0,0,400,0};
    DrawTextW(hdc, currentMessage, -1, &textRect, DT_CALCRECT|DT_WORDBREAK);
    int w = min(textRect.right + 20, 400);
    int h = min(textRect.bottom + 20, 300);
    ReleaseDC(NULL, hdc);

    // 화면 우하단 위치
    int x = GetSystemMetrics(SM_CXSCREEN) - w - 20;
    int y = GetSystemMetrics(SM_CYSCREEN) - h - 60;

    // 창 생성
    hPopupWnd = CreateWindowExW(
        WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE|WS_EX_LAYERED,
        WINDOW_CLASS_NAME, L"", WS_POPUP,
        x,y,w,h, hOwner, NULL, GetModuleHandle(NULL), NULL
    );

    SetLayeredWindowAttributes(hPopupWnd, 0, (255*TRANSPARENCY_LEVEL)/100, LWA_ALPHA);
    ShowWindow(hPopupWnd, SW_SHOWNOACTIVATE);
    SetTimer(hPopupWnd, 1, POPUP_LIFETIME_MS, NULL);
    UpdateWindow(hPopupWnd);
}

// 트레이 아이콘 초기화
void AddTrayIcon(HWND hwnd) {
    nid.cbSize = sizeof(nid);
    nid.hWnd   = hwnd;
    nid.uID    = TRAY_ICON_UID;
    nid.uFlags = NIF_ICON|NIF_MESSAGE|NIF_TIP;
    nid.uCallbackMessage = WM_TRAYICON;
    nid.hIcon  = LoadIcon(NULL, IDI_INFORMATION);
    wcscpy(nid.szTip, L"Clipflash");
    Shell_NotifyIcon(NIM_ADD, &nid);
}

int WINAPI wWinMain(
    HINSTANCE hInst,
    HINSTANCE hPrevInstance,
    PWSTR    pCmdLine,
    int      nCmdShow
) {
    // 윈도우 클래스 등록
    WNDCLASSW wc = { .lpfnWndProc=WndProc, .hInstance=hInst,
        .lpszClassName=WINDOW_CLASS_NAME,
        .hbrBackground=CreateSolidBrush(RGB(255,255,255)),
        .hCursor=LoadCursor(NULL, IDC_ARROW)
    };
    RegisterClassW(&wc);

    // 히든 윈도우 + 트레이
    HWND hWnd = CreateWindowExW(0, WINDOW_CLASS_NAME, L"Main",
        WS_OVERLAPPEDWINDOW, CW_USEDEFAULT,CW_USEDEFAULT,100,100,
        NULL,NULL,hInst,NULL);
    AddTrayIcon(hWnd);

    MSG msg;
    while (1) {
        // 1) 클립보드 열기
        if (OpenClipboard(NULL)) {
            HANDLE hData = GetClipboardData(CF_UNICODETEXT);
            if (hData) {
                WCHAR *clipText = (WCHAR*)GlobalLock(hData);
                if (clipText) {
                    // 2) 첫 실행 또는 변경 감지
                    if (!lastText || wcscmp(clipText, lastText)!=0) {
                        // (a) 이전 메모리 해제
                        if (lastText) free(lastText);
                        // (b) 전체 길이만큼 할당·복사
                        size_t len = (wcslen(clipText)+1)*sizeof(WCHAR);
                        lastText = malloc(len);
                        memcpy(lastText, clipText, len);
                        // (c) 팝업 호출
                        ShowPopup(hWnd, clipText);
                    }
                    GlobalUnlock(hData);
                }
            }
            CloseClipboard();
        }

        // 3) 메시지 처리
        while (PeekMessage(&msg, NULL,0,0,PM_REMOVE)) {
            if (msg.message==WM_QUIT) goto END;
            TranslateMessage(&msg);
            DispatchMessage(&msg);
        }

        Sleep(POLL_INTERVAL_MS);
    }

END:
    CleanupTrayIcon();
    free(lastText);
    return 0;
}
