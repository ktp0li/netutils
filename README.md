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
PING ya.ru (77.88.44.242) with 12(40) bytes of data
20 bytes from ya.ru (77.88.44.242): icmp_seq=1 ttl=245 time=13.96 ms
20 bytes from ya.ru (77.88.44.242): icmp_seq=2 ttl=245 time=16.39 ms
20 bytes from ya.ru (77.88.44.242): icmp_seq=3 ttl=245 time=17.30 ms
^C
--- 77.88.44.242 ping statistics ---
4 packets transmitted, 3 received, 25% packet loss, 3002 ms
rtt min/avg/max/mdev = 13.962/15.886/17.303/1.410 ms
~ ❯ sudo ./ping ya.ru --privileged -c 5
PING ya.ru (5.255.255.242) with 12(40) bytes of data
20 bytes from ya.ru (5.255.255.242): icmp_seq=1 ttl=244 time=14.73 ms
20 bytes from ya.ru (5.255.255.242): icmp_seq=2 ttl=244 time=17.54 ms
20 bytes from ya.ru (5.255.255.242): icmp_seq=3 ttl=244 time=17.66 ms
20 bytes from ya.ru (5.255.255.242): icmp_seq=4 ttl=244 time=17.28 ms
20 bytes from ya.ru (5.255.255.242): icmp_seq=5 ttl=244 time=17.03 ms

--- 5.255.255.242 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, 5003 ms
rtt min/avg/max/mdev = 14.732/16.848/17.655/1.080 ms
```

example with packet loss:
```
~ ❯ sudo tc qdisc add dev wlan0 root netem loss 50% 25% 
~ ❯ ./ping 8.8.8.8 -c 5
PING 8.8.8.8 (8.8.8.8) with 12(40) bytes of data
20 bytes from 8.8.8.8: icmp_seq=1 ttl=110 time=9.60 ms
20 bytes from 8.8.8.8: icmp_seq=3 ttl=110 time=9.20 ms

--- 8.8.8.8 ping statistics ---
5 packets transmitted, 2 received, 60% packet loss, 5002 ms
rtt min/avg/max/mdev = 9.204/9.404/9.604/0.200 ms
```
TODO:
- maybe parse another packet types? (host unreachable etc)