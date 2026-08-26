package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

func resolveAddress(flagAddress string) (string, error) {
	address := strings.TrimSpace(flagAddress)
	if address == "" {
		if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
			port, err := strconv.Atoi(raw)
			if err != nil || port < 1 || port > 65535 {
				return "", fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
			}
			address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		} else {
			address = defaultAddress
		}
	}
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("监听地址格式无效: %w", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("监听端口必须在 1 到 65535 之间")
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return "", fmt.Errorf("监听地址必须使用回环主机")
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
