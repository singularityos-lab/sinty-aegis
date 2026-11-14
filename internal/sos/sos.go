package sos

import (
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
)

var ErrNoLine = errors.New("no phone line on this computer")
var ErrNotEmergency = errors.New("not an emergency number here")

const fallback = "112"

func Numbers(country string) []string {
	if n, ok := numbers[strings.ToLower(country)]; ok {
		return append([]string(nil), n...)
	}
	return []string{fallback}
}

func Known(country, number string) bool {
	for _, n := range Numbers(country) {
		if n == number {
			return true
		}
	}
	return false
}

type Line struct {
	Path  dbus.ObjectPath
	Model string
}

const mm = "org.freedesktop.ModemManager1"

func Lines(bus *dbus.Conn) []Line {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := bus.Object(mm, "/org/freedesktop/ModemManager1").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	if err != nil {
		return nil
	}
	var out []Line
	for path, ifaces := range objects {
		if _, voice := ifaces[mm+".Modem.Voice"]; !voice {
			continue
		}
		model := ""
		if v, ok := ifaces[mm+".Modem"]["Model"]; ok {
			model, _ = v.Value().(string)
		}
		out = append(out, Line{path, model})
	}
	return out
}

func Call(bus *dbus.Conn, country, number string) (dbus.ObjectPath, error) {
	if !Known(country, number) {
		return "", ErrNotEmergency
	}
	lines := Lines(bus)
	if len(lines) == 0 {
		return "", ErrNoLine
	}
	modem := bus.Object(mm, lines[0].Path)
	var call dbus.ObjectPath
	props := map[string]dbus.Variant{"number": dbus.MakeVariant(number)}
	if err := modem.Call(mm+".Modem.Voice.CreateCall", 0, props).Store(&call); err != nil {
		return "", err
	}
	if err := bus.Object(mm, call).Call(mm+".Call.Start", 0).Err; err != nil {
		return "", err
	}
	return call, nil
}
