package asset

import (
	log "dsiem/internal/pkg/shared/logger"
	"dsiem/internal/pkg/shared/str"

	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/yl2chen/cidranger"
)

const (
	assetsFileGlob = "assets_*.json"
)

var ranger cidranger.Ranger

type networkAsset struct {
	Name  string `json:"name"`
	Cidr  string `json:"cidr"`
	Value int    `json:"value"`
}

type networkAssets struct {
	NetworkAssets []networkAsset `json:"assets"`
}

var assets networkAssets

type assetEntry struct {
	ipNet net.IPNet
	value int
	name  string
}

func (b *assetEntry) Network() net.IPNet {
	return b.ipNet
}

func newAssetEntry(ipNet net.IPNet, value int, name string) cidranger.RangerEntry {
	return &assetEntry{
		ipNet: ipNet,
		value: value,
		name:  name,
	}
}

// Init read assets from all asset_* files in confDir
func Init(confDir string) error {
	p := path.Join(confDir, assetsFileGlob)
	files, _ := filepath.Glob(p)
	if len(files) == 0 {
		return errors.New("Cannot find asset files in " + p)
	}

	for i := range files {
		var a networkAssets
		file, err := os.Open(files[i])
		if err != nil {
			return err
		}
		defer file.Close()

		byteValue, _ := ioutil.ReadAll(file)
		err = json.Unmarshal(byteValue, &a)
		if err != nil {
			log.Info(log.M{Msg: "Cannot unmarshal asset!"})
			return err
		}
		for j := range a.NetworkAssets {
			assets.NetworkAssets = append(assets.NetworkAssets, a.NetworkAssets[j])
		}
	}

	ranger = cidranger.NewPCTrieRanger()

	for i := range assets.NetworkAssets {
		cidr := assets.NetworkAssets[i].Cidr
		value := assets.NetworkAssets[i].Value
		name := assets.NetworkAssets[i].Name

		_, net, err := net.ParseCIDR(cidr)
		if err != nil {
			// log.Info("Cannot parse "+cidr+"!", 0)
			log.Info(log.M{Msg: "Cannot parse " + cidr + "!"})
			return err
		}

		if value == 0 || name == "" {
			return errors.New("value cannot be 0 and name cannot be empty for " + cidr)
		}

		_ = ranger.Insert(newAssetEntry(*net, value, name))
	}

	total := len(assets.NetworkAssets)

	log.Info(log.M{Msg: "Loaded " + strconv.Itoa(total) + " host and/or network assets."})

	return nil
}

// IsInHomeNet check if IP is in HOME_NET
func IsInHomeNet(ip string) (bool, error) {
	contains, err := ranger.Contains(net.ParseIP(ip)) // returns true, nil
	return contains, err
}

// GetName returns the asset name
func GetName(ip string) string {
	val := ""
	containingNetworks, err := ranger.ContainingNetworks(net.ParseIP(ip))
	if err != nil || len(containingNetworks) == 0 {
		return val
	}
	// return the one with /32
	for i := range containingNetworks {
		r := containingNetworks[i].(*assetEntry)
		m := r.ipNet.Mask.String()
		if m == "ffffffff" {
			val = r.name
			break
		}
	}
	return val
}

// GetValue returns asset value
func GetValue(ip string) int {
	val := 0
	containingNetworks, err := ranger.ContainingNetworks(net.ParseIP(ip))
	if err != nil || len(containingNetworks) == 0 {
		return val
	}
	// return the highest asset value
	for i := range containingNetworks {
		r, ok := containingNetworks[i].(*assetEntry)
		if ok && r.value > val {
			val = r.value
		}
	}
	return val
}

// GetAssetNetworks return the CIDR network that the IP is in
func GetAssetNetworks(ip string) []string {
	val := []string{}
	containingNetworks, err := ranger.ContainingNetworks(net.ParseIP(ip))
	if err != nil || len(containingNetworks) == 0 {
		return val
	}
	// return all network string except those with /32
	for i := range containingNetworks {
		r := containingNetworks[i].(*assetEntry)
		m := r.ipNet.Mask.String()
		if m != "ffffffff" {
			s := r.ipNet.String()
			val = str.AppendUniq(val, s)
		}
	}
	return val
}
