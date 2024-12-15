# my implementation of network utilities
## ping

### oh, what is whis for?
my goal is to create a simple ping implementation without using `net/icmp` package. i find it funny, im enjoying building packages from scratch :)

---

build:

```
make ping
```

usage:

```
~ ❯ ./ping --help
Usage of ./ping:
  -c, --count int    stop after <count> replies
  -d, --debug        
      --privileged   run with ip:icmp socket instead udp
~ ❯ ./ping ya.ru -c 5
PING ya.ru (77.88.55.242) with TODO bytes of data
20 bytes from 77.88.55.242: icmp_seq=1 ttl=246 time=17 ms
20 bytes from 77.88.55.242: icmp_seq=2 ttl=246 time=19 ms
20 bytes from 77.88.55.242: icmp_seq=3 ttl=246 time=19 ms
20 bytes from 77.88.55.242: icmp_seq=4 ttl=246 time=19 ms
20 bytes from 77.88.55.242: icmp_seq=5 ttl=246 time=19 ms
~ ❯ sudo ./ping ya.ru -c 5 --privileged
PING ya.ru (77.88.55.242) with TODO bytes of data
20 bytes from 77.88.55.242: icmp_seq=1 ttl=246 time=17 ms
20 bytes from 77.88.55.242: icmp_seq=2 ttl=246 time=20 ms
20 bytes from 77.88.55.242: icmp_seq=3 ttl=246 time=19 ms
20 bytes from 77.88.55.242: icmp_seq=4 ttl=246 time=19 ms
20 bytes from 77.88.55.242: icmp_seq=5 ttl=246 time=17 ms

```
TODO:
- add ip packet size/icmp data size to heading (PING <...> **56(84) bytes** of data.)
- add summary after ping (statistic)
- make two threads for sending/receiving packets
- add identifier to icmp payload