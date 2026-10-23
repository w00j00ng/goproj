#include <assert.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/socketvar.h>
#include <sys/sysctl.h>
#include <sys/proc_info.h>
#include <libproc.h>
#include <arpa/inet.h>
#include <errno.h>
#include <netinet/in_pcb.h>
#include <netinet/tcp.h>
#include <netinet/tcp_fsm.h>
#include <netinet/tcp_var.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "native_darwin.h"

enum { SYSCTL_OK, SYSCTL_GROW_ONCE, SYSCTL_ALWAYS_GROWS, SYSCTL_DENIED, SYSCTL_TOO_LARGE };
enum { SOCKET_OK, SOCKET_SHORT, SOCKET_DENIED };
static int sysctl_mode, socket_mode, size_calls, read_calls;
static const void *table_data;
static size_t table_length;
static struct socket_fdinfo socket_data;

int pw_test_sysctlbyname(const char *name, void *buffer, size_t *length, void *new_value, size_t new_length) {
    assert(strcmp(name, "net.inet.tcp.pcblist64") == 0);
    assert(new_value == NULL && new_length == 0 && errno == 0);
    if (buffer == NULL) {
        size_calls++;
        if (sysctl_mode == SYSCTL_DENIED) { errno = EACCES; return -1; }
        *length = sysctl_mode == SYSCTL_TOO_LARGE ? (size_t)64 * 1024 * 1024 + 1 : table_length;
        return 0;
    }
    read_calls++;
    if (sysctl_mode == SYSCTL_ALWAYS_GROWS || (sysctl_mode == SYSCTL_GROW_ONCE && read_calls == 1)) {
        *length = table_length + 1024;
        errno = ENOMEM;
        return -1;
    }
    assert(*length >= table_length);
    memcpy(buffer, table_data, table_length);
    *length = table_length;
    return 0;
}

int pw_test_proc_listpids(uint32_t type, uint32_t type_info, void *buffer, int capacity) {
    assert(type == PROC_ALL_PIDS && type_info == 0 && errno == 0);
    int32_t pid = 1234;
    if (buffer == NULL) return sizeof(pid);
    assert(capacity >= (int)sizeof(pid));
    memcpy(buffer, &pid, sizeof(pid));
    return sizeof(pid);
}

int pw_test_proc_pidinfo(int pid, int flavor, uint64_t arg, void *buffer, int capacity) {
    assert(pid == 1234 && flavor == PROC_PIDLISTFDS && arg == 0 && errno == 0);
    struct proc_fdinfo fd = {.proc_fd = 7, .proc_fdtype = PROX_FDTYPE_SOCKET};
    if (buffer == NULL) return sizeof(fd);
    assert(capacity >= (int)sizeof(fd));
    memcpy(buffer, &fd, sizeof(fd));
    return sizeof(fd);
}

int pw_test_proc_pidfdinfo(int pid, int fd, int flavor, void *buffer, int capacity) {
    assert(pid == 1234 && fd == 7 && flavor == PROC_PIDFDSOCKETINFO && errno == 0);
    assert(capacity == (int)sizeof(socket_data));
    if (socket_mode == SOCKET_DENIED) { errno = EACCES; return 0; }
    memcpy(buffer, &socket_data, sizeof(socket_data));
    return socket_mode == SOCKET_SHORT ? (int)sizeof(socket_data) - 1 : (int)sizeof(socket_data);
}

int pw_test_proc_pidpath(int pid, void *buffer, uint32_t capacity) {
    static const char path[] = "/tmp/listener";
    assert(pid == 1234 && capacity >= sizeof(path) && errno == 0);
    memcpy(buffer, path, sizeof(path));
    return sizeof(path) - 1;
}

static void check_collect(int mode, int expected, int expected_reads, int expected_errno) {
    sysctl_mode = mode;
    size_calls = read_calls = 0;
    pw_socket *rows = NULL;
    size_t count = 0;
    int os_error = -1;
    errno = ERANGE;
    assert(pw_collect(&rows, &count, &os_error) == expected);
    assert(read_calls == expected_reads && os_error == expected_errno);
    assert(size_calls == (expected_reads == 0 ? 1 : expected_reads));
    if (expected == PW_OK) assert(count == 1);
    else assert(rows == NULL && count == 0);
    free(rows);
}

static void check_native_calls(const void *data, size_t length) {
    table_data = data;
    table_length = length;
    check_collect(SYSCTL_GROW_ONCE, PW_OK, 2, 0);
    check_collect(SYSCTL_ALWAYS_GROWS, PW_RESOURCE_LIMIT, 5, 0);
    check_collect(SYSCTL_TOO_LARGE, PW_RESOURCE_LIMIT, 0, 0);
    check_collect(SYSCTL_DENIED, PW_OS_ERROR, 0, EACCES);

    socket_data.psi.soi_so = 11;
    socket_data.psi.soi_pcb = 22;
    socket_data.psi.soi_kind = SOCKINFO_TCP;
    socket_data.psi.soi_type = SOCK_STREAM;
    socket_data.psi.soi_protocol = IPPROTO_TCP;
    socket_data.psi.soi_family = AF_INET6;
    socket_data.psi.soi_proto.pri_tcp.tcpsi_state = TSI_S_LISTEN;
    struct in_sockinfo *inp = &socket_data.psi.soi_proto.pri_tcp.tcpsi_ini;
    inp->insi_vflag = INI_IPV6;
    inp->insi_lport = htons(8080);
    inp->insi_gencnt = 8;
    assert(inet_pton(AF_INET6, "::ffff:127.0.0.1", &inp->insi_laddr.ina_6) == 1);

    pw_socket socket;
    int os_error = -1;
    socket_mode = SOCKET_SHORT;
    errno = ERANGE;
    assert(pw_socketinfo(1234, 7, &socket, &os_error) == PW_INVALID_DATA && os_error == 0);
    socket_mode = SOCKET_DENIED;
    errno = ERANGE;
    assert(pw_socketinfo(1234, 7, &socket, &os_error) == PW_OS_ERROR && os_error == EACCES);
    socket_mode = SOCKET_OK;
    assert(pw_socketinfo(1234, 7, &socket, &os_error) == PW_OK);
    assert(socket.socket_token == 11 && socket.pcb_token == 22 && socket.generation == 8);
    assert(socket.family == 6 && socket.port == 8080);

    // Successful libproc reads cannot change a failed shared-table acquisition.
    int32_t *pids = NULL, *fds = NULL;
    size_t count = 0;
    assert(pw_listpids(&pids, &count, &os_error) == PW_OK && count == 1 && pids[0] == 1234);
    assert(pw_listfds(pids[0], &fds, &count, &os_error) == PW_OK && count == 1 && fds[0] == 7);
    assert(pw_socketinfo(pids[0], fds[0], &socket, &os_error) == PW_OK);
    char *path = NULL;
    assert(pw_imagepath((uint32_t)pids[0], &path, &os_error) == PW_OK && strcmp(path, "/tmp/listener") == 0);
    free(pids);
    free(fds);
    free(path);
    check_collect(SYSCTL_DENIED, PW_OS_ERROR, 0, EACCES);
}

static void check(const void *data, size_t length, int expected, size_t count) {
    pw_socket *rows = NULL;
    size_t actual = 0;
    assert(pw_parse_pcblist(data, length, &rows, &actual) == expected);
    if (expected == PW_OK) assert(actual == count);
    else assert(rows == NULL && actual == 0);
    free(rows);
}

int main(void) {
    struct xinpgen head = {0};
    head.xig_len = sizeof(head);
    head.xig_gen = 9;
    head.xig_sogen = 10;
    check(&head, sizeof(head), PW_OK, 0);
    check(NULL, 0, PW_INVALID_DATA, 0);
    check(&head, sizeof(head) - 1, PW_INVALID_DATA, 0);

    head.xig_count = 1;
    struct xtcpcb64 tcp = {0};
    tcp.xt_len = sizeof(tcp);
    tcp.xt_inpcb.xi_len = sizeof(tcp.xt_inpcb);
    tcp.xt_inpcb.xi_socket.xso_len = sizeof(tcp.xt_inpcb.xi_socket);
    tcp.xt_inpcb.xi_socket.xso_family = AF_INET6;
    tcp.xt_inpcb.xi_socket.xso_protocol = IPPROTO_TCP;
    tcp.xt_inpcb.xi_socket.so_type = SOCK_STREAM;
    tcp.xt_inpcb.xi_socket.xso_so = 11;
    tcp.xt_inpcb.xi_socket.so_pcb = 22;
    tcp.xt_inpcb.inp_gencnt = 8;
    tcp.xt_inpcb.inp_vflag = INP_IPV6;
    tcp.xt_inpcb.inp_lport = htons(8080);
    tcp.t_state = TCPS_LISTEN;
    assert(inet_pton(AF_INET6, "fe80:4::1", &tcp.xt_inpcb.in6p_laddr) == 1);
    unsigned char data[2 * sizeof(head) + sizeof(tcp)];
    memcpy(data, &head, sizeof(head));
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    memcpy(data + sizeof(head) + sizeof(tcp), &head, sizeof(head));
    check(data, sizeof(data), PW_OK, 1);
    pw_socket *rows = NULL;
    size_t count = 0;
    assert(pw_parse_pcblist(data, sizeof(data), &rows, &count) == PW_OK);
    assert(rows[0].family == 6 && rows[0].port == 8080 && rows[0].scope_id == 4);
    assert(rows[0].address[2] == 0 && rows[0].address[3] == 0);
    assert(rows[0].socket_token == 11 && rows[0].pcb_token == 22 && rows[0].generation == 8);
    free(rows);

    assert(inet_pton(AF_INET6, "2001:db8:4::1", &tcp.xt_inpcb.in6p_laddr) == 1);
    tcp.xt_inpcb.inp_depend6.inp6_ifindex = 7;
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    assert(pw_parse_pcblist(data, sizeof(data), &rows, &count) == PW_OK);
    assert(rows[0].address[2] == 0x0d && rows[0].address[3] == 0xb8 && rows[0].scope_id == 0);
    free(rows);

    assert(inet_pton(AF_INET6, "::ffff:127.0.0.1", &tcp.xt_inpcb.in6p_laddr) == 1);
    tcp.xt_inpcb.inp_vflag = INP_IPV4;
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    assert(pw_parse_pcblist(data, sizeof(data), &rows, &count) == PW_OK);
    assert(rows[0].family == 6 && rows[0].address[10] == 0xff && rows[0].address[11] == 0xff);
    free(rows);

    check(data, sizeof(data) - sizeof(head), PW_INVALID_DATA, 0);
    check(data, sizeof(data) - 1, PW_INVALID_DATA, 0);
    struct xinpgen tail = head;
    tail.xig_gen++;
    memcpy(data + sizeof(head) + sizeof(tcp), &tail, sizeof(tail));
    check(data, sizeof(data), PW_CHANGED, 0);
    memcpy(data + sizeof(head) + sizeof(tcp), &head, sizeof(head));
    tcp.xt_len = 0;
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    check(data, sizeof(data), PW_INVALID_DATA, 0);
    tcp.xt_len = sizeof(tcp);
    tcp.xt_inpcb.xi_len--;
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    check(data, sizeof(data), PW_INVALID_DATA, 0);
    tcp.xt_inpcb.xi_len = sizeof(tcp.xt_inpcb);
    tcp.xt_inpcb.xi_socket.xso_len--;
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    check(data, sizeof(data), PW_INVALID_DATA, 0);
    tcp.xt_inpcb.xi_socket.xso_len = sizeof(tcp.xt_inpcb.xi_socket);
    tcp.xt_inpcb.inp_gencnt = head.xig_gen + 1;
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    check(data, sizeof(data), PW_CHANGED, 0);
    tcp.xt_inpcb.inp_gencnt = 8;
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    check_native_calls(data, sizeof(data));
    tcp.t_state = TCPS_ESTABLISHED;
    memcpy(data + sizeof(head), &tcp, sizeof(tcp));
    check(data, sizeof(data), PW_OK, 0);
    return 0;
}
