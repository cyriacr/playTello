#!/bin/bash
file="wifi.lst"
digit=2
while IFS=: read -r interface essid key
do
	iwconfig $interface essid "$essid" key "$key"
	newip=192.168.10.$digit
	digit=$((digit+1))
	ifconfig $interface $newip netmask 255.255.255.0
done <"$file"


