#define _GNU_SOURCE             /* See feature_test_macros(7) */
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/fcntl.h>
#include <sys/types.h>
#include <unistd.h>
#include <ifaddrs.h>

// NF impl which may require dyn link. Not ideal for reproducible build
//#include <netlink/socket.h>
//#include <netlink/route/rtnl.h>
//#include <netlink/route/link.h>
//
//void dump_info() {
//    struct nl_sock* sock = nl_socket_alloc();
//    if (!sock) {
//        perror("allocatenl");
//        exit(1);
//    }
//    nl_connect(sock, NETLINK_ROUTE);
//
//    struct nl_cache *cache = NULL;
//    int ret = rtnl_link_alloc_cache(sock, AF_UNSPEC, &cache);
//    if (ret < 0) {
//        printf("%p %p\n", cache, sock);
//        nl_perror(ret, "alloc cache");
//        exit(1);
//    }
//
//    int nitem = nl_cache_nitems(cache);
//    printf("there are %d items in cache\n", nitem);
//
//    struct nl_object* obj = nl_cache_get_first(cache);
//    while (obj) {
//        struct rtnl_link* l = (struct rtnl_link*) obj;
//        /// things are included in netlink-private/types.h, which are not included
//        //printf("link name %s\n", l->l_name);
//        printf("link name %s, index %d\n", rtnl_link_get_name(l), rtnl_link_get_ifindex(l));
//        obj = nl_cache_get_next(obj);
//    }
//    nl_cache_put(cache);
//}
//
//
#include <arpa/inet.h>
#include <netinet/in.h>
void dump_info() {
    struct ifaddrs *addrs, *cur;
    getifaddrs(&addrs);
    cur = addrs;
    while (cur) {
        char buf[INET_ADDRSTRLEN];
        if (cur->ifa_addr->sa_family == AF_INET) {
            struct sockaddr_in* addr = (struct sockaddr_in*) cur->ifa_addr;
            inet_ntop(AF_INET, &addr->sin_addr, buf, INET_ADDRSTRLEN);
            printf("%s %s\n", cur->ifa_name, buf);
        }
        cur = cur->ifa_next;
    }
    freeifaddrs(addrs);
}

int main(int argc, char **argv) {

    if (argc < 2) {
        dump_info();
        return 0;
    }

    int i ;
    for (i = 1; i < argc; i++) {
        int fd = open(argv[i], O_RDONLY);
        if (fd < 0) {
            perror("open");
            return 2;
        }
        int err = setns(fd, CLONE_NEWNET);
        if (err < 0) {
            perror("error in joining");
            return 2;
        }
        dump_info();
    }

    return 0;
}
