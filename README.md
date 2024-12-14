# my implementation of network utilities
## ping
### build:
```
make ping
```
### usage:


```
~ ❯ sudo ./ping --help
Usage of ./ping:
  -c, --count int   stop after <count> replies
  -d, --debug
~ ❯ sudo ./ping ya.ru -c 5
PING 77.88.55.242
20 bytes from 77.88.55.242: icmp_seq=1, time=21 ms
20 bytes from 77.88.55.242: icmp_seq=2, time=22 ms
20 bytes from 77.88.55.242: icmp_seq=3, time=21 ms
20 bytes from 77.88.55.242: icmp_seq=4, time=21 ms
20 bytes from 77.88.55.242: icmp_seq=5, time=21 ms
```