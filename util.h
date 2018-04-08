int connect_with_timeout(int socket, const struct sockaddr *address,
                         socklen_t address_len, int timeout);
int connect_to_host(const char *host, unsigned short port, int timeout);
ssize_t xread(int fd, void *buf, size_t len);
ssize_t xwrite(int fd, const void *buf, size_t len);

size_t _strlcpy(char * __restrict dst, const char * __restrict src, size_t siz);
#ifndef strlcpy
#define strlcpy _strlcpy
#endif
