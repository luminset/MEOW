//go:build generate
// +build generate

// go run chinaip_gen.go

package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// use china ip list database by ipip.net
const (
	chinaIPListFile  = "https://github.com/17mon/china_ip_list/raw/master/china_ip_list.txt"
	chinaIP6ListFile = "https://github.com/17mon/china_ip_list/raw/master/china_ipv6_list.txt"
)

func main() {
	startList := []string{}
	countList := []string{}

	resp, err := http.Get(chinaIPListFile)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		panic(fmt.Errorf("Unexpected status %d", resp.StatusCode))
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			panic(errors.New("Invalid CIDR"))
		}
		ip := parts[0]
		mask := parts[1]
		count, err := cidrCalc(mask)
		if err != nil {
			panic(err)
		}

		ipLong, err := ipToUint32(ip)
		if err != nil {
			panic(err)
		}
		startList = append(startList, strconv.FormatUint(uint64(ipLong), 10))
		countList = append(countList, strconv.FormatUint(uint64(count), 10))
	}

	startList6High := []string{}
	startList6Low := []string{}
	countList6 := []string{}

	resp6, err := http.Get(chinaIP6ListFile)
	if err != nil {
		log.Printf("Warning: failed to fetch IPv6 list: %v", err)
	} else {
		if resp6.StatusCode != 200 {
			log.Printf("Warning: unexpected status %d for IPv6 list", resp6.StatusCode)
		} else {
			defer resp6.Body.Close()
			scanner6 := bufio.NewScanner(resp6.Body)
			for scanner6.Scan() {
				line := scanner6.Text()
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.Split(line, "/")
				if len(parts) != 2 {
					panic(errors.New("Invalid IPv6 CIDR"))
				}
				ip := parts[0]
				mask := parts[1]
				count, err := cidrCalc6(mask)
				if err != nil {
					panic(err)
				}

				high, low, err := ipToUint128(ip)
				if err != nil {
					panic(err)
				}
				startList6High = append(startList6High, strconv.FormatUint(high, 10))
				startList6Low = append(startList6Low, strconv.FormatUint(low, 10))
				countList6 = append(countList6, strconv.FormatUint(uint64(count), 10))
			}
		}
	}

	file, err := os.OpenFile("chinaip_data.go", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("Failed to generate chinaip_data.go: %v", err)
	}
	defer file.Close()

	fmt.Fprintln(file, "package main")
	fmt.Fprint(file, "var CNIPDataStart = []uint32 {\n	")
	fmt.Fprint(file, strings.Join(startList, ",\n	"))
	fmt.Fprintln(file, ",\n	}")

	fmt.Fprint(file, "var CNIPDataNum = []uint{\n	")
	fmt.Fprint(file, strings.Join(countList, ",\n	"))
	fmt.Fprintln(file, ",\n	}")

	fmt.Fprint(file, "\nvar CNIPDataStart6High = []uint64 {\n	")
	fmt.Fprint(file, strings.Join(startList6High, ",\n	"))
	fmt.Fprintln(file, ",\n	}")

	fmt.Fprint(file, "var CNIPDataStart6Low = []uint64 {\n	")
	fmt.Fprint(file, strings.Join(startList6Low, ",\n	"))
	fmt.Fprintln(file, ",\n	}")

	fmt.Fprint(file, "var CNIPDataNum6 = []uint{\n	")
	fmt.Fprint(file, strings.Join(countList6, ",\n	"))
	fmt.Fprintln(file, ",\n	}")
}

func cidrCalc(mask string) (uint, error) {
	i, err := strconv.Atoi(mask)
	if err != nil || i > 32 {
		return 0, errors.New("Invalid Mask")
	}
	p := 32 - i
	res := uint(intPow2(p))
	return res, nil
}

func intPow2(p int) int {
	r := 1
	for i := 0; i < p; i++ {
		r *= 2
	}
	return r
}

func ipToUint32(ipstr string) (uint32, error) {
	ip := net.ParseIP(ipstr)
	if ip == nil {
		return 0, errors.New("Invalid IP")
	}
	ip = ip.To4()
	if ip == nil {
		return 0, errors.New("Not IPv4")
	}
	return binary.BigEndian.Uint32(ip), nil
}

func cidrCalc6(mask string) (uint, error) {
	i, err := strconv.Atoi(mask)
	if err != nil || i > 128 {
		return 0, errors.New("Invalid Mask")
	}
	p := 128 - i
	res := uint(intPow2(p))
	return res, nil
}

func ipToUint128(ipstr string) (uint64, uint64, error) {
	ip := net.ParseIP(ipstr)
	if ip == nil {
		return 0, 0, errors.New("Invalid IP")
	}
	ip = ip.To16()
	if ip == nil {
		return 0, 0, errors.New("Not IPv6")
	}
	high := binary.BigEndian.Uint64(ip[0:8])
	low := binary.BigEndian.Uint64(ip[8:16])
	return high, low, nil
}
