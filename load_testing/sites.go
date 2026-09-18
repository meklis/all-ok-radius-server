package main

import (
	"fmt"
	"math/rand"
)

// Site - профиль одного NAS/релея, каким он виден в реальном трафике
// (см. примеры tcpdump в задаче): свой NAS-IP, NAS-Identifier и
// Called-Station-Id, свой способ формирования NAS-Port.
type Site struct {
	NasIP           string
	NasID           string
	CalledStationID string

	// LocalPort=true - маленький положительный NAS-Port + NAS-Port-Id
	// (как у "LOCAL_20"/"Loc_95" реле), false - большое "отрицательное"
	// значение NAS-Port без NAS-Port-Id (как у большинства DHCP-relay).
	LocalPort  bool
	PortID     string
	PortNumber int
}

// baseSites - профили, снятые непосредственно с реальных пакетов из задачи.
var baseSites = []Site{
	{NasIP: "10.27.2.100", NasID: "CCR-27", CalledStationID: "Relay_27_local"},
	{NasIP: "10.43.100.139", NasID: "s39-Ra3", CalledStationID: "dhcp_relay"},
	{NasIP: "10.38.100.1", NasID: "s38-Dr16", CalledStationID: "dhcp_relay"},
	{NasIP: "10.20.2.100", NasID: "CCR-20", CalledStationID: "Relay_20_local", LocalPort: true, PortID: "LOCAL_20", PortNumber: 49},
	{NasIP: "10.120.0.100", NasID: "NAT_120", CalledStationID: "server-inet"},
	{NasIP: "10.50.200.100", NasID: "50-Kolc13a-NAT", CalledStationID: "DHCP-TWRT"},
	{NasIP: "10.128.100.1", NasID: "CCR-128", CalledStationID: "Relay_28_local", LocalPort: true, PortID: "LOCAL_28", PortNumber: 28},
	{NasIP: "10.35.200.100", NasID: "35-M34-NAT", CalledStationID: "DHCP-TWRT"},
	{NasIP: "10.22.2.100", NasID: "CCR-22", CalledStationID: "Relay_22_local"},
	{NasIP: "10.95.100.1", NasID: "s95-Ar5", CalledStationID: "dhcp_relay", LocalPort: true, PortID: "Loc_95", PortNumber: 33},
	{NasIP: "10.67.200.100", NasID: "67-To1-NAT", CalledStationID: "dhcp_relay"},
	{NasIP: "10.254.242.4", NasID: "131-SG35-NAT", CalledStationID: "DHCP-TWRT"},
}

var calledStationOptions = []string{"dhcp_relay", "DHCP-TWRT", "server-inet"}

// buildSitePool возвращает пул NAS-профилей для теста: реальные примеры плюс
// extra синтетических, сгенерированных по тем же паттернам (разные "районы").
func buildSitePool(extra int, rnd *rand.Rand) []Site {
	sites := make([]Site, len(baseSites))
	copy(sites, baseSites)

	octets2 := []int{2, 100, 200}
	octets3 := []int{1, 100}
	for i := 0; i < extra; i++ {
		district := rnd.Intn(250) + 2
		ip := fmt.Sprintf("10.%d.%d.%d", district, octets2[rnd.Intn(len(octets2))], octets3[rnd.Intn(len(octets3))])

		var s Site
		s.NasIP = ip
		if rnd.Intn(3) == 0 {
			s.NasID = fmt.Sprintf("CCR-%d", district)
			s.CalledStationID = fmt.Sprintf("Relay_%d_local", district)
			s.LocalPort = true
			s.PortID = fmt.Sprintf("LOCAL_%d", district)
			s.PortNumber = rnd.Intn(48) + 1
		} else {
			s.NasID = fmt.Sprintf("s%d-Ar%d", district, rnd.Intn(9)+1)
			s.CalledStationID = calledStationOptions[rnd.Intn(len(calledStationOptions))]
		}
		sites = append(sites, s)
	}
	return sites
}
