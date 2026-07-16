package main

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	nethttp "net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const defaultQQWryUpdateURL = "https://raw.githubusercontent.com/FW27623/qqwry/main/qqwry.dat"

type qqwryDB struct {
	data    []byte
	first   uint32
	last    uint32
	count   uint32
	modTime time.Time
}

var qqwryState struct {
	sync.RWMutex
	db *qqwryDB
}

func initQQWryData() {
	if config.QQWryFile == "" {
		return
	}
	if err := loadQQWryData(); err != nil {
		errl.Printf("QQWry.dat load failed: %v, fallback to built-in china ip list and update in background", err)
		go func() {
			if err := updateQQWryData(); err != nil {
				errl.Printf("QQWry.dat update failed: %v", err)
			}
		}()
	}
	if config.QQWryUpdateInterval > 0 && config.QQWryUpdateURL != "" {
		go updateQQWryPeriodically()
	}
}

func updateQQWryPeriodically() {
	ticker := time.NewTicker(config.QQWryUpdateInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := updateQQWryData(); err != nil {
			errl.Printf("QQWry.dat scheduled update failed: %v", err)
		}
	}
}

func loadQQWryData() error {
	db, err := openQQWry(config.QQWryFile)
	if err != nil {
		return err
	}
	qqwryState.Lock()
	qqwryState.db = db
	qqwryState.Unlock()
	debug.Printf("Load QQWry.dat: %s", config.QQWryFile)
	return nil
}

func updateQQWryData() error {
	if config.QQWryUpdateURL == "" {
		return errors.New("qqwryUpdateURL is empty")
	}
	if err := os.MkdirAll(config.dir, 0755); err != nil {
		return err
	}
	tmpFile := config.QQWryFile + ".download"
	client := &nethttp.Client{Timeout: 3 * time.Minute}
	resp, err := client.Get(config.QQWryUpdateURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != nethttp.StatusOK {
		return errors.New("unexpected status: " + resp.Status)
	}
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(tmpFile)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(tmpFile)
		return closeErr
	}
	if _, err = openQQWry(tmpFile); err != nil {
		os.Remove(tmpFile)
		return err
	}
	if err = os.Rename(tmpFile, config.QQWryFile); err != nil {
		os.Remove(tmpFile)
		return err
	}
	errl.Printf("QQWry.dat updated: %s", config.QQWryFile)
	return loadQQWryData()
}

func openQQWry(file string) (*qqwryDB, error) {
	info, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if len(data) < 8 {
		return nil, errors.New("QQWry.dat is too small")
	}
	first := binary.LittleEndian.Uint32(data[0:4])
	last := binary.LittleEndian.Uint32(data[4:8])
	if first < 8 || last < first || int(last)+7 > len(data) {
		return nil, errors.New("QQWry.dat index range invalid")
	}
	return &qqwryDB{
		data:    data,
		first:   first,
		last:    last,
		count:   (last-first)/7 + 1,
		modTime: info.ModTime(),
	}, nil
}

func shouldUseQQWry() bool {
	qqwryState.RLock()
	db := qqwryState.db
	qqwryState.RUnlock()
	if db == nil {
		return false
	}
	if info, err := os.Stat(config.CNIPFile); err == nil {
		return !db.modTime.Before(info.ModTime())
	}
	return true
}

func qqwryShouldDirect(ip string) (direct, ok bool) {
	qqwryState.RLock()
	db := qqwryState.db
	qqwryState.RUnlock()
	if db == nil {
		return false, false
	}
	return db.shouldDirect(ip)
}

func (db *qqwryDB) shouldDirect(ip string) (direct, ok bool) {
	ipLong, err := ipToUint32(ip)
	if err != nil {
		return false, false
	}
	recordOffset, found := db.findRecord(ipLong)
	if !found {
		return false, false
	}
	if int(recordOffset)+4 > len(db.data) {
		return false, false
	}
	endIP := binary.LittleEndian.Uint32(db.data[recordOffset : recordOffset+4])
	if ipLong > endIP {
		return false, false
	}
	country, area := db.readLocation(recordOffset + 4)
	return isChinaQQWryLocation(country + " " + area)
}

func (db *qqwryDB) findRecord(ipLong uint32) (uint32, bool) {
	if db.count == 0 {
		return 0, false
	}
	var low uint32
	high := db.count - 1
	var candidate uint32
	found := false
	for low <= high {
		mid := (low + high) / 2
		indexOffset := db.first + mid*7
		if int(indexOffset)+7 > len(db.data) {
			return 0, false
		}
		startIP := binary.LittleEndian.Uint32(db.data[indexOffset : indexOffset+4])
		if startIP <= ipLong {
			candidate = readQQWryUint24(db.data[indexOffset+4 : indexOffset+7])
			found = true
			low = mid + 1
		} else {
			if mid == 0 {
				break
			}
			high = mid - 1
		}
	}
	return candidate, found
}

func (db *qqwryDB) readLocation(offset uint32) (country, area string) {
	if int(offset) >= len(db.data) {
		return "", ""
	}
	mode := db.data[offset]
	switch mode {
	case 1:
		if int(offset)+4 > len(db.data) {
			return "", ""
		}
		redirect := readQQWryUint24(db.data[offset+1 : offset+4])
		if int(redirect) >= len(db.data) {
			return "", ""
		}
		if db.data[redirect] == 2 {
			if int(redirect)+4 > len(db.data) {
				return "", ""
			}
			countryOffset := readQQWryUint24(db.data[redirect+1 : redirect+4])
			country = db.readString(countryOffset)
			area = db.readArea(redirect + 4)
			return
		}
		country, offset = db.readStringWithEnd(redirect)
		area = db.readArea(offset)
	case 2:
		if int(offset)+4 > len(db.data) {
			return "", ""
		}
		countryOffset := readQQWryUint24(db.data[offset+1 : offset+4])
		country = db.readString(countryOffset)
		area = db.readArea(offset + 4)
	default:
		country, offset = db.readStringWithEnd(offset)
		area = db.readArea(offset)
	}
	return
}

func (db *qqwryDB) readArea(offset uint32) string {
	if int(offset) >= len(db.data) {
		return ""
	}
	mode := db.data[offset]
	if mode == 1 || mode == 2 {
		if int(offset)+4 > len(db.data) {
			return ""
		}
		return db.readString(readQQWryUint24(db.data[offset+1 : offset+4]))
	}
	return db.readString(offset)
}

func (db *qqwryDB) readString(offset uint32) string {
	s, _ := db.readStringWithEnd(offset)
	return s
}

func (db *qqwryDB) readStringWithEnd(offset uint32) (string, uint32) {
	if int(offset) >= len(db.data) {
		return "", offset
	}
	end := offset
	for int(end) < len(db.data) && db.data[end] != 0 {
		end++
	}
	raw := db.data[offset:end]
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
	if err != nil {
		return string(raw), end + 1
	}
	return string(decoded), end + 1
}

func readQQWryUint24(b []byte) uint32 {
	if len(b) < 3 {
		return 0
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

func isChinaQQWryLocation(location string) (direct, ok bool) {
	location = strings.TrimSpace(location)
	if location == "" || strings.Contains(location, "CZ88.NET") {
		return false, false
	}
	if strings.Contains(location, "局域网") ||
		strings.Contains(location, "本机") ||
		strings.Contains(location, "保留地址") ||
		strings.Contains(location, "IANA") {
		return true, true
	}
	for _, term := range chinaLocationTerms {
		if strings.Contains(location, term) {
			return true, true
		}
	}
	return false, true
}

var chinaLocationTerms = []string{
	"中国", "北京", "上海", "天津", "重庆", "河北", "山西", "辽宁", "吉林", "黑龙江",
	"江苏", "浙江", "安徽", "福建", "江西", "山东", "河南", "湖北", "湖南", "广东",
	"海南", "四川", "贵州", "云南", "陕西", "甘肃", "青海", "台湾", "内蒙古", "广西",
	"西藏", "宁夏", "新疆", "香港", "澳门",
}

func isIPv4String(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() != nil
}
