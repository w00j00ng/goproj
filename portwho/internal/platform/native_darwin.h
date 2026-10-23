#ifndef PORTWHO_NATIVE_DARWIN_H
#define PORTWHO_NATIVE_DARWIN_H

#include <stddef.h>
#include <stdint.h>

enum { PW_OK, PW_OS_ERROR, PW_RESOURCE_LIMIT, PW_INVALID_DATA, PW_CHANGED, PW_NOT_LISTENER };

typedef struct {
    uint64_t socket_token;
    uint64_t pcb_token;
    uint64_t generation;
    uint8_t address[16];
    uint32_t scope_id;
    uint16_t port;
    uint8_t family;
} pw_socket;

int pw_parse_pcblist(const void *data, size_t length, pw_socket **rows, size_t *count);
int pw_collect(pw_socket **rows, size_t *count, int *os_error);
int pw_listpids(int32_t **pids, size_t *count, int *os_error);
int pw_listfds(int32_t pid, int32_t **fds, size_t *count, int *os_error);
int pw_socketinfo(int32_t pid, int32_t fd, pw_socket *socket, int *os_error);
int pw_imagepath(uint32_t pid, char **path, int *os_error);
void pw_free(void *data);

#endif
