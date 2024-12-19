# my implementation of network utilities
## ping

### oh, what is whis for?
my goal is to create a simple ping implementation without using `net/icmp` package. i find it funny, im enjoying building network packets from scratch :)

---

build:

```
make ping
```

usage:

```
~ ❯ ./ping --help
Usage of ./ping:
  -c, --count int          stop after <count> replies (default -1)
      --data bytesBase64   data to transfer in package body (default aGV5eQ==)
  -d, --debug              print debug messages
      --delay duration     delay between sending packets (default 1s)
      --privileged         run with ip:icmp socket instead udp
~ ❯ ./ping ya.ru
PING ya.ru (5.255.255.242) with 12(40) bytes of data
20 bytes from ya.ru (5.255.255.242): icmp_seq=1 ttl=244 time=15.51 ms
20 bytes from ya.ru (5.255.255.242): icmp_seq=2 ttl=244 time=18.04 ms
20 bytes from ya.ru (5.255.255.242): icmp_seq=3 ttl=244 time=17.09 ms
^C
--- 5.255.255.242 ping statistics ---
4 packets transmitted, 3 received, 25% packet loss, 3001 ms
rtt min/avg/max/mdev = 15.510/5.627/18.038/1.043 ms
~ ❯ sudo ./ping ya.ru --privileged -c 5
PING ya.ru (77.88.55.242) with 12(40) bytes of data
20 bytes from ya.ru (77.88.55.242): icmp_seq=1 ttl=53 time=18.31 ms
20 bytes from ya.ru (77.88.55.242): icmp_seq=2 ttl=53 time=20.86 ms
20 bytes from ya.ru (77.88.55.242): icmp_seq=3 ttl=53 time=20.76 ms
20 bytes from ya.ru (77.88.55.242): icmp_seq=4 ttl=53 time=18.07 ms
20 bytes from ya.ru (77.88.55.242): icmp_seq=5 ttl=53 time=20.42 ms

--- 77.88.55.242 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, 5002 ms
rtt min/avg/max/mdev = 18.069/3.937/20.855/1.230 ms

```

example with packet loss:
```
~ ❯ sudo tc qdisc add dev wlan0 root netem loss 50% 25% 
~ ❯ ./ping 8.8.8.8 -c 5
PING 8.8.8.8 (8.8.8.8) with 12(40) bytes of data
20 bytes from 8.8.8.8: icmp_seq=1 ttl=110 time=9.32 ms
20 bytes from 8.8.8.8: icmp_seq=3 ttl=110 time=9.62 ms

--- 8.8.8.8 ping statistics ---
5 packets transmitted, 2 received, 60% packet loss, 5002 ms
rtt min/avg/max/mdev = 9.318/4.733/9.615/0.149 ms
```
TODO:
- maybe parse another packet types? (host unreachable etc)