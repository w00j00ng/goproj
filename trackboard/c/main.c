#include <windows.h>
#include <wchar.h>
#include <shellapi.h>

#define MAX_CLIP_LEN 8192
#define POPUP_LIFETIME_MS 1000
#define WINDOW_CLASS_NAME L"ClipboardPopupWindow"
#define TRAY_ICON_UID 1
#define WM_TRAYICON (WM_USER + 1)
#define TRANSPARENCY_LEVEL 50
#define SLEEP_TIME_MS 10

WCHAR lastText[MAX_CLIP_LEN] = L"";
HWND hPopupWnd = NULL;
WCHAR currentMessage[MAX_CLIP_LEN] = L"";
HFONT hFont = NULL;
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

            POINT pt;
            GetCursorPos(&pt);
            SetForegroundWindow(hwnd); // 필수
            int cmd = TrackPopupMenu(hMenu, TPM_RETURNCMD | TPM_NONOTIFY, pt.x, pt.y, 0, hwnd, NULL);
            if (cmd == 1) {
                CleanupTrayIcon();
                PostQuitMessage(0);
            }
        }
        break;
    case WM_PAINT: {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);

        SetBkMode(hdc, TRANSPARENT);
        SetTextColor(hdc, RGB(30, 30, 30));
        SelectObject(hdc, hFont);

        RECT rect;
        GetClientRect(hwnd, &rect);
        InflateRect(&rect, -10, -10);

        DrawTextW(hdc, currentMessage, -1, &rect, DT_WORDBREAK | DT_LEFT | DT_TOP);

        EndPaint(hwnd, &ps);
        break;
    }
    default:
        return DefWindowProcW(hwnd, msg, wParam, lParam);
    }
    return 0;
}

void ShowPopup(HWND hOwner, const WCHAR* text) {
    if (hPopupWnd != NULL) {
        DestroyWindow(hPopupWnd);
    }

    wcsncpy(currentMessage, text, MAX_CLIP_LEN - 1);
    currentMessage[MAX_CLIP_LEN - 1] = L'\0';

    HDC hdc = GetDC(NULL);
    if (!hFont) {
        hFont = CreateFontW(-16, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE,
            DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS,
            CLEARTYPE_QUALITY, DEFAULT_PITCH | FF_DONTCARE, L"Segoe UI");
    }
    SelectObject(hdc, hFont);

    // 텍스트 크기 측정
    RECT textRect = { 0, 0, 400, 0 };
    DrawTextW(hdc, currentMessage, -1, &textRect, DT_CALCRECT | DT_WORDBREAK);
    int width = textRect.right + 20;
    int height = textRect.bottom + 20;
    if (width > 400) width = 400;
    if (height > 300) height = 300;
    ReleaseDC(NULL, hdc);

    // 위치 계산
    int x = GetSystemMetrics(SM_CXSCREEN) - width - 20;
    int y = GetSystemMetrics(SM_CYSCREEN) - height - 60;

    hPopupWnd = CreateWindowExW(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE | WS_EX_LAYERED,
        WINDOW_CLASS_NAME, L"",
        WS_POPUP,
        x, y, width, height,
        hOwner, NULL, GetModuleHandle(NULL), NULL
    );

    SetLayeredWindowAttributes(hPopupWnd, 0, (255 * TRANSPARENCY_LEVEL) / 100, LWA_ALPHA);

    ShowWindow(hPopupWnd, SW_SHOWNOACTIVATE);
    SetTimer(hPopupWnd, 1, POPUP_LIFETIME_MS, NULL);
    UpdateWindow(hPopupWnd);
}

void AddTrayIcon(HWND hwnd) {
    nid.cbSize = sizeof(nid);
    nid.hWnd = hwnd;
    nid.uID = TRAY_ICON_UID;
    nid.uFlags = NIF_ICON | NIF_MESSAGE | NIF_TIP;
    nid.uCallbackMessage = WM_TRAYICON;
    nid.hIcon = LoadIcon(NULL, IDI_INFORMATION);
    wcscpy(nid.szTip, L"클립보드 팝업 감시기");

    Shell_NotifyIcon(NIM_ADD, &nid);
}

int WINAPI wWinMain(HINSTANCE hInstance, HINSTANCE hPrevInstance, PWSTR pCmdLine, int nCmdShow) {
    WNDCLASSW wc = { 0 };
    wc.lpfnWndProc = WndProc;
    wc.hInstance = hInstance;
    wc.lpszClassName = WINDOW_CLASS_NAME;
    wc.hbrBackground = CreateSolidBrush(RGB(255, 255, 255));
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    RegisterClassW(&wc);

    HWND hHiddenWindow = CreateWindowExW(0, WINDOW_CLASS_NAME, L"Main", WS_OVERLAPPEDWINDOW,
        CW_USEDEFAULT, CW_USEDEFAULT, 100, 100, NULL, NULL, hInstance, NULL);
    AddTrayIcon(hHiddenWindow);

    MSG msg;
    while (1) {
        if (OpenClipboard(NULL)) {
            HANDLE hData = GetClipboardData(CF_UNICODETEXT);
            if (hData) {
                WCHAR* clipText = (WCHAR*)GlobalLock(hData);
                if (clipText) {
                    if (wcscmp(clipText, lastText) != 0) {
                        wcsncpy(lastText, clipText, MAX_CLIP_LEN - 1);
                        lastText[MAX_CLIP_LEN - 1] = L'\0';
                        ShowPopup(hHiddenWindow, clipText);
                    }
                    GlobalUnlock(hData);
                }
            }
            CloseClipboard();
        }

        while (PeekMessage(&msg, NULL, 0, 0, PM_REMOVE)) {
            if (msg.message == WM_QUIT) goto end;
            TranslateMessage(&msg);
            DispatchMessage(&msg);
        }

        Sleep(SLEEP_TIME_MS);
    }

end:
    CleanupTrayIcon();
    return 0;
}