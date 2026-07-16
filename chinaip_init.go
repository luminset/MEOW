//go:generate go run chinaip_gen.go

package main

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/cyfdecyf/bufio"
)

// data range by first byte for IPv4
var CNIPDataRange [256]struct {
	start int
	end   int
}

// data range by first byte for IPv6
var CNIPDataRange6 [256]struct {
	start int
	end   int
}

func initCNIPData() {
	importCNIPFile()

	n := len(CNIPDataStart)
	var curr uint32
	var preFirstByte uint32
	for i := 0; i < n; i++ {
		firstByte := CNIPDataStart[i] >> 24
		if curr != firstByte {
			curr = firstByte
			if preFirstByte != 0 {
				CNIPDataRange[preFirstByte].end = i - 1
			}
			CNIPDataRange[firstByte].start = i
			preFirstByte = firstByte
		}
	}
	if n > 0 {
		CNIPDataRange[preFirstByte].end = n - 1
	}

	n6 := len(CNIPDataStart6High)
	var curr6 uint64
	var preFirstByte6 uint64
	for i := 0; i < n6; i++ {
		firstByte6 := CNIPDataStart6High[i] >> 56
		if curr6 != firstByte6 {
			curr6 = firstByte6
			if preFirstByte6 != 0 {
				CNIPDataRange6[preFirstByte6].end = i - 1
			}
			CNIPDataRange6[firstByte6].start = i
			preFirstByte6 = firstByte6
		}
	}
	if n6 > 0 {
		CNIPDataRange6[preFirstByte6].end = n6 - 1
	}
}

func importCNIPFile() {
	if err := isFileExists(config.CNIPFile); err != nil {
		return
	}
	f, err := os.Open(config.CNIPFile)
	if err != nil {
		errl.Println("Error opening china ip list:", err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	CNIPDataStart = []uint32{}
	CNIPDataNum = []uint{}
	CNIPDataStart6High = []uint64{}
	CNIPDataStart6Low = []uint64{}
	CNIPDataNum6 = []uint{}

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			panic(errors.New("Invalid CIDR Format"))
		}
		ip := parts[0]
		mask := parts[1]

		ipObj := net.ParseIP(ip)
		if ipObj == nil {
			panic(errors.New("Invalid IP: " + ip))
		}

		if ipObj.To4() != nil {
			count, err := cidrCalc(mask)
			if err != nil {
				panic(err)
			}
			ipLong, err := ipToUint32(ip)
			if err != nil {
				panic(err)
			}
			CNIPDataStart = append(CNIPDataStart, ipLong)
			CNIPDataNum = append(CNIPDataNum, count)
		} else {
			count, err := cidrCalc6(mask)
			if err != nil {
				panic(err)
			}
			high, low, err := ipToUint128(ip)
			if err != nil {
				panic(err)
			}
			CNIPDataStart6High = append(CNIPDataStart6High, high)
			CNIPDataStart6Low = append(CNIPDataStart6Low, low)
			CNIPDataNum6 = append(CNIPDataNum6, count)
		}
	}
	debug.Printf("Load china ip list")
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
