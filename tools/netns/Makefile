
.phony: clean all


all: ipshow

ipshow: ipshow.c
	#gcc -g $^ -o $@ -I/usr/include/libnl3 -lnl-3 -lnl-route-3
	gcc -g $^ -o $@ -I/usr/include/libnl3 -static

clean:
	rm -f ipshow
