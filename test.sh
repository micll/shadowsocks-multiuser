#!/usr/bin/env bash
go build -ldflags "-w -s"
systemctl stop shadowsocks-multiuser
cp shadowsocks-multiuser /opt/shadowsocks-multiuser/shadowsocks-multiuser
systemctl restart shadowsocks-multiuser