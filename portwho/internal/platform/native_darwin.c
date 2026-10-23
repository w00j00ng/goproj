//go:build darwin && cgo && (amd64 || arm64)

#include "native_darwin.h"

#include <sys/types.h>
#include <sys/socket.h>
#include <sys/socketvar.h>
#include <sys/sysctl.h>
#include <netinet/in.h>
#include <netinet/in_pcb.h>
#include <netinet/tcp.h>
#include <netinet/tcp_fsm.h>
#include <netinet/tcp_var.h>
#include <sys/proc_info.h>
#include <libproc.h>
#include <arpa/inet.h>
#include <errno.h>
#include <limits.h>
#include <stdlib.h>
#include <string.h>

#define PW_MAX_BUFFER ((size_t)64 * 1024 * 1024)
#define PW_MAX_ATTEMPTS 5

static void normalize_scope(pw_socket *socket) {
    struct in6_addr address;
    memcpy(&address, socket->address, sizeof(address));
    if (IN6_IS_ADDR_LINKLOCAL(&address) || IN6_IS_ADDR_MC_LINKLOCAL(&address) ||
        IN6_IS_ADDR_MC_NODELOCAL(&address)) {
        uint32_t embedded = ((uint32_t)socket->address[2] << 8) | socket->address[3];
        if (embedded != 0) socket->scope_id = embedded;
        socket->address[2] = socket->address[3] = 0;
    } else socket->scope_id = 0;
}

static int from_pcb(const struct xtcpcb64 *tcp, pw_socket *out) {
    const struct xinpcb64 *inp = &tcp->xt_inpcb;
    const struct xsocket64 *socket = &inp->xi_socket;
    if (tcp->t_state != TCPS_LISTEN || socket->so_type != SOCK_STREAM ||
        socket->xso_protocol != IPPROTO_TCP) return PW_NOT_LISTENER;
    memset(out, 0, sizeof(*out));
    if (!(inp->inp_vflag & (INP_IPV4 | INP_IPV6))) return PW_INVALID_DATA;
    if (socket->xso_family == AF_INET) {
        out->family = 4;
        memcpy(out->address, &inp->inp_laddr, 4);
    } else if (socket->xso_family == AF_INET6) {
        out->family = 6;
        memcpy(out->address, &inp->in6p_laddr, 16);
        out->scope_id = inp->inp_depend6.inp6_ifindex;
        normalize_scope(out);
    } else {
        return PW_INVALID_DATA;
    }
    out->port = ntohs(inp->inp_lport);
    out->socket_token = socket->xso_so;
    out->pcb_token = socket->so_pcb;
    out->generation = inp->inp_gencnt;
    return PW_OK;
}

int pw_parse_pcblist(const void *data, size_t length, pw_socket **rows, size_t *count) {
    *rows = NULL;
    *count = 0;
    if (length > PW_MAX_BUFFER) return PW_RESOURCE_LIMIT;
    if (data == NULL || length < sizeof(struct xinpgen)) return PW_INVALID_DATA;
    struct xinpgen first;
    memcpy(&first, data, sizeof(first));
    if (first.xig_len != sizeof(first)) return PW_INVALID_DATA;
    // XNU returns only the leading header when the PCB count is zero.
    if (length == sizeof(first)) return first.xig_count == 0 ? PW_OK : PW_INVALID_DATA;
    size_t capacity = length / sizeof(struct xtcpcb64);
    pw_socket *result = capacity ? calloc(capacity, sizeof(*result)) : NULL;
    if (capacity && result == NULL) return PW_RESOURCE_LIMIT;
    size_t offset = sizeof(first), used = 0, records = 0;
    int status = PW_INVALID_DATA;
    while (offset < length) {
        uint32_t record_length;
        if (length - offset < sizeof(record_length)) break;
        memcpy(&record_length, (const char *)data + offset, sizeof(record_length));
        if (record_length == sizeof(struct xinpgen)) {
            if (length - offset != sizeof(struct xinpgen)) break;
            struct xinpgen last;
            memcpy(&last, (const char *)data + offset, sizeof(last));
            if (last.xig_gen != first.xig_gen || last.xig_sogen != first.xig_sogen ||
                last.xig_count != first.xig_count) {
                status = PW_CHANGED;
                break;
            }
            *rows = result;
            *count = used;
            return PW_OK;
        }
        if (record_length != sizeof(struct xtcpcb64) || record_length > length - offset) break;
        struct xtcpcb64 tcp;
        memcpy(&tcp, (const char *)data + offset, sizeof(tcp));
        if (tcp.xt_inpcb.xi_len != sizeof(tcp.xt_inpcb) ||
            tcp.xt_inpcb.xi_socket.xso_len != sizeof(tcp.xt_inpcb.xi_socket)) break;
        if (++records > first.xig_count) break;
        if (tcp.xt_inpcb.inp_gencnt > first.xig_gen) {
            status = PW_CHANGED;
            break;
        }
        if (used >= capacity) break;
        int decoded = from_pcb(&tcp, &result[used]);
        if (decoded == PW_OK) used++;
        else if (decoded != PW_NOT_LISTENER) break;
        offset += record_length;
    }
    free(result);
    return status;
}

int pw_collect(pw_socket **rows, size_t *count, int *os_error) {
    *rows = NULL;
    *count = 0;
    *os_error = 0;
    for (int attempt = 0; attempt < PW_MAX_ATTEMPTS; attempt++) {
        size_t capacity = 0;
        errno = 0;
        int result = sysctlbyname("net.inet.tcp.pcblist64", NULL, &capacity, NULL, 0);
        int saved_errno = errno;
        if (result != 0) { *os_error = saved_errno; return PW_OS_ERROR; }
        if (capacity > PW_MAX_BUFFER) return PW_RESOURCE_LIMIT;
        if (capacity < sizeof(struct xinpgen)) return PW_INVALID_DATA;
        void *buffer = malloc(capacity);
        if (buffer == NULL) return PW_RESOURCE_LIMIT;
        size_t length = capacity;
        errno = 0;
        result = sysctlbyname("net.inet.tcp.pcblist64", buffer, &length, NULL, 0);
        saved_errno = errno;
        if (result != 0) {
            free(buffer);
            if (saved_errno == ENOMEM) continue;
            *os_error = saved_errno;
            return PW_OS_ERROR;
        }
        if (length > capacity) { free(buffer); return PW_INVALID_DATA; }
        int status = pw_parse_pcblist(buffer, length, rows, count);
        free(buffer);
        if (status != PW_CHANGED) return status;
    }
    return PW_RESOURCE_LIMIT;
}

static int read_list(int32_t pid, int is_fds, void **data, size_t *count, int *os_error) {
    *data = NULL;
    *count = 0;
    *os_error = 0;
    size_t unit = is_fds ? sizeof(struct proc_fdinfo) : sizeof(int32_t);
    errno = 0;
    int needed = is_fds ? proc_pidinfo(pid, PROC_PIDLISTFDS, 0, NULL, 0) :
        proc_listpids(PROC_ALL_PIDS, 0, NULL, 0);
    int saved_errno = errno;
    if (needed < 0 || (needed == 0 && saved_errno != 0)) { *os_error = saved_errno; return PW_OS_ERROR; }
    if (needed == 0) return PW_OK;
    size_t capacity = (size_t)needed;
    if (capacity % unit != 0) return PW_INVALID_DATA;
    for (int attempt = 0; attempt < PW_MAX_ATTEMPTS; attempt++) {
        if (capacity > PW_MAX_BUFFER - 32 * unit) return PW_RESOURCE_LIMIT;
        capacity += 32 * unit;
        void *buffer = malloc(capacity);
        if (buffer == NULL) return PW_RESOURCE_LIMIT;
        errno = 0;
        int length = is_fds ? proc_pidinfo(pid, PROC_PIDLISTFDS, 0, buffer, (int)capacity) :
            proc_listpids(PROC_ALL_PIDS, 0, buffer, (int)capacity);
        saved_errno = errno;
        if (length < 0 || (length == 0 && saved_errno != 0)) {
            free(buffer); *os_error = saved_errno; return PW_OS_ERROR;
        }
        if ((size_t)length > capacity || (size_t)length % unit != 0) { free(buffer); return PW_INVALID_DATA; }
        if ((size_t)length >= capacity - unit) {
            free(buffer);
            if (capacity > PW_MAX_BUFFER / 2) return PW_RESOURCE_LIMIT;
            capacity *= 2;
            continue;
        }
        *data = buffer;
        *count = (size_t)length / unit;
        return PW_OK;
    }
    return PW_RESOURCE_LIMIT;
}

int pw_listpids(int32_t **pids, size_t *count, int *os_error) {
    void *data = NULL;
    int status = read_list(0, 0, &data, count, os_error);
    *pids = data;
    return status;
}

int pw_listfds(int32_t pid, int32_t **fds, size_t *count, int *os_error) {
    *fds = NULL;
    void *data = NULL;
    size_t total = 0;
    int status = read_list(pid, 1, &data, &total, os_error);
    *count = 0;
    if (status != PW_OK) return status;
    // Compact the native FD records in place after reading each complete record.
    struct proc_fdinfo *native = data;
    int32_t *result = data;
    for (size_t i = 0; i < total; i++) {
        struct proc_fdinfo fd;
        memcpy(&fd, &native[i], sizeof(fd));
        if (fd.proc_fdtype != PROX_FDTYPE_SOCKET) continue;
        if (fd.proc_fd < 0) { free(data); return PW_INVALID_DATA; }
        result[(*count)++] = (int32_t)fd.proc_fd;
    }
    *fds = result;
    return PW_OK;
}

int pw_socketinfo(int32_t pid, int32_t fd, pw_socket *socket, int *os_error) {
    struct socket_fdinfo native;
    memset(&native, 0, sizeof(native));
    *os_error = 0;
    errno = 0;
    int length = proc_pidfdinfo(pid, fd, PROC_PIDFDSOCKETINFO, &native, sizeof(native));
    int saved_errno = errno;
    if (length <= 0) { *os_error = saved_errno; return PW_OS_ERROR; }
    if ((size_t)length != sizeof(native)) return PW_INVALID_DATA;
    const struct socket_info *info = &native.psi;
    const struct in_sockinfo *inp = &info->soi_proto.pri_tcp.tcpsi_ini;
    if (info->soi_kind != SOCKINFO_TCP || info->soi_type != SOCK_STREAM ||
        info->soi_protocol != IPPROTO_TCP || info->soi_proto.pri_tcp.tcpsi_state != TSI_S_LISTEN) return PW_NOT_LISTENER;
    memset(socket, 0, sizeof(*socket));
    if (!(inp->insi_vflag & (INI_IPV4 | INI_IPV6))) return PW_INVALID_DATA;
    if (info->soi_family == AF_INET) {
        socket->family = 4;
        memcpy(socket->address, &inp->insi_laddr.ina_46.i46a_addr4, 4);
    } else if (info->soi_family == AF_INET6) {
        socket->family = 6;
        memcpy(socket->address, &inp->insi_laddr.ina_6, 16);
        socket->scope_id = inp->insi_v6.in6_ifindex;
        normalize_scope(socket);
    } else return PW_INVALID_DATA;
    socket->port = ntohs((uint16_t)inp->insi_lport);
    socket->socket_token = info->soi_so;
    socket->pcb_token = info->soi_pcb;
    socket->generation = inp->insi_gencnt;
    return PW_OK;
}

int pw_imagepath(uint32_t pid, char **path, int *os_error) {
    *path = NULL;
    *os_error = 0;
    if (pid > INT_MAX) { *os_error = EINVAL; return PW_OS_ERROR; }
    char *buffer = calloc(PROC_PIDPATHINFO_MAXSIZE, 1);
    if (buffer == NULL) return PW_RESOURCE_LIMIT;
    errno = 0;
    int length = proc_pidpath((int)pid, buffer, PROC_PIDPATHINFO_MAXSIZE);
    int saved_errno = errno;
    if (length <= 0) { free(buffer); *os_error = saved_errno; return PW_OS_ERROR; }
    if (length >= PROC_PIDPATHINFO_MAXSIZE || memchr(buffer, 0, PROC_PIDPATHINFO_MAXSIZE) == NULL) {
        free(buffer); return PW_INVALID_DATA;
    }
    *path = buffer;
    return PW_OK;
}

void pw_free(void *data) { free(data); }
