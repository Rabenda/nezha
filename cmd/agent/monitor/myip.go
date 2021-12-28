package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/naiba/nezha/pkg/utils"
)

type geoIP struct {
	CountryCode string `json:"country_code,omitempty"`
	IP          string `json:"ip,omitempty"`
	Query       string `json:"query,omitempty"`
}

var (
	geoIPApiList = []string{
		"https://api.ip.sb/geoip",
		"https://ip.seeip.org/geoip",
		"https://ipapi.co/json",
		"https://freegeoip.app/json/",
		"http://ip-api.com/json/",
		"https://extreme-ip-lookup.com/json/",
	}
	cachedIP, cachedCountry string
	httpClientV4            = utils.NewSingleStackHTTPClient(time.Second*20, time.Second*5, time.Second*10, false)
	httpClientV6            = utils.NewSingleStackHTTPClient(time.Second*20, time.Second*5, time.Second*10, true)
)

func UpdateIP() {
	for {
		if ip, _ := getClientIp(); ip != "" {
			cachedIP = fmt.Sprintf("%s", ip)
		}

		time.Sleep(time.Minute * 30)
	}
}

func fetchGeoIP(servers []string, isV6 bool) geoIP {
	var ip geoIP
	var resp *http.Response
	var err error
	for i := 0; i < len(servers); i++ {
		if isV6 {
			resp, err = httpClientV6.Get(servers[i])
		} else {
			resp, err = httpClientV4.Get(servers[i])
		}
		if err == nil {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			resp.Body.Close()
			err = json.Unmarshal(body, &ip)
			if err != nil {
				continue
			}
			if ip.IP == "" && ip.Query != "" {
				ip.IP = ip.Query
			}
			// 没取到 v6 IP
			if isV6 && !strings.Contains(ip.IP, ":") {
				continue
			}
			// 没取到 v4 IP
			if !isV6 && !strings.Contains(ip.IP, ".") {
				continue
			}
			return ip
		}
	}
	return ip
}

func getClientIp() (string, error) {
	addrs, err := net.InterfaceAddrs()

	if err != nil {
		return "", err
	}

	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && isPrivate(ipnet.IP) {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", errors.New("Can not find the client ip address!")
}

func isPrivate(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		// Following RFC 1918, Section 3. Private Address Space which says:
		//   The Internet Assigned Numbers Authority (IANA) has reserved the
		//   following three blocks of the IP address space for private internets:
		//     10.0.0.0        -   10.255.255.255  (10/8 prefix)
		//     172.16.0.0      -   172.31.255.255  (172.16/12 prefix)
		//     192.168.0.0     -   192.168.255.255 (192.168/16 prefix)
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1]&0xf0 == 16) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}
	// IPv6 Drop
	return false
	// Following RFC 4193, Section 8. IANA Considerations which says:
	//   The IANA has assigned the FC00::/7 prefix to "Unique Local Unicast".
	// return len(ip) == IPv6len && ip[0]&0xfe == 0xfc
}
