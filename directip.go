package main

import (
	"strings"
)

func ipShouldDirect(ip string) (direct bool) {
	direct = false
	defer func() {
		if r := recover(); r != nil {
			errl.Printf("error judging ip should direct: %s", ip)
		}
	}()
	_, isPrivate := hostIsIP(ip)
	if isPrivate {
		return true
	}

	if strings.Contains(ip, ":") {
		ipHigh, ipLow, err := ip2long6(ip)
		if err != nil {
			return false
		}
		if ipHigh == 0 && ipLow == 0 {
			return true
		}
		firstByte := ipHigh >> 56
		if CNIPDataRange6[firstByte].end == 0 && CNIPDataRange6[firstByte].start == 0 {
			return false
		}
		ipIndex := searchRange(CNIPDataRange6[firstByte].start, CNIPDataRange6[firstByte].end, func(i int) bool {
			if CNIPDataStart6High[i] > ipHigh {
				return true
			}
			if CNIPDataStart6High[i] == ipHigh && CNIPDataStart6Low[i] > ipLow {
				return true
			}
			return false
		})
		ipIndex--
		if ipIndex < 0 {
			return false
		}
		if CNIPDataStart6High[ipIndex] > ipHigh {
			return false
		}
		if CNIPDataStart6High[ipIndex] < ipHigh {
			return true
		}
		return ipLow <= CNIPDataStart6Low[ipIndex]+(uint64)(CNIPDataNum6[ipIndex])
	}

	ipLong, err := ip2long(ip)
	if err != nil {
		return false
	}
	if ipLong == 0 {
		return true
	}
	firstByte := ipLong >> 24
	if CNIPDataRange[firstByte].end == 0 && CNIPDataRange[firstByte].start == 0 {
		return false
	}
	ipIndex := searchRange(CNIPDataRange[firstByte].start, CNIPDataRange[firstByte].end, func(i int) bool {
		return CNIPDataStart[i] > ipLong
	})
	ipIndex--
	if ipIndex < 0 {
		return false
	}
	return ipLong <= CNIPDataStart[ipIndex]+(uint32)(CNIPDataNum[ipIndex])
}
