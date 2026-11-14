package service

import (
	"encoding/json"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"

	"github.com/singularityos-lab/sinty-aegis/internal/alert"
	"github.com/singularityos-lab/sinty-aegis/internal/config"
	"github.com/singularityos-lab/sinty-aegis/internal/sos"
)

const Name = "dev.sinty.Aegis1"
const Path = dbus.ObjectPath("/dev/sinty/Aegis1")

type Bus struct {
	engine  *Engine
	session *dbus.Conn
	system  *dbus.Conn
}

type wireAlert struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Severity      string `json:"severity"`
	Headline      string `json:"headline"`
	Description   string `json:"description"`
	Instruction   string `json:"instruction"`
	Area          string `json:"area"`
	Source        string `json:"source"`
	Link          string `json:"link"`
	Sent          int64  `json:"sent"`
	Expires       int64  `json:"expires"`
	Raised        int64  `json:"raised"`
	DismissableAt int64  `json:"dismissable_at"`
	Test          bool   `json:"test"`
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func encode(a alert.Alert) wireAlert {
	return wireAlert{a.ID, string(a.Kind), a.Severity.String(), a.Headline, a.Description, a.Instruction, a.Area, a.Source, a.Link, unix(a.Sent), unix(a.Expires), unix(a.Raised), unix(a.DismissableAt()), a.Test}
}

func fail(err error) *dbus.Error {
	return dbus.NewError(Name+".Error", []interface{}{err.Error()})
}

func (b *Bus) Alerts() (string, *dbus.Error) {
	active := b.engine.Store.Active()
	out := make([]wireAlert, 0, len(active))
	for _, a := range active {
		out = append(out, encode(a))
	}
	data, _ := json.Marshal(out)
	return string(data), nil
}

func (b *Bus) Dismiss(id string) *dbus.Error {
	if err := b.engine.Store.Dismiss(id); err != nil {
		return fail(err)
	}
	return nil
}

func (b *Bus) Test(kind string) (string, *dbus.Error) {
	return b.engine.Test(alert.Kind(kind)).ID, nil
}

func (b *Bus) Config() (string, *dbus.Error) {
	data, _ := json.Marshal(b.engine.Config())
	return string(data), nil
}

func (b *Bus) SetConfig(doc string) *dbus.Error {
	c := config.Default()
	if err := json.Unmarshal([]byte(doc), &c); err != nil {
		return fail(err)
	}
	if err := b.engine.SetConfig(c); err != nil {
		return fail(err)
	}
	b.Changed()
	return nil
}

func (b *Bus) Areas() ([]string, *dbus.Error) {
	return b.engine.Areas(), nil
}

func (b *Bus) Sos() (string, *dbus.Error) {
	cfg := b.engine.Config()
	country, here, located := cfg.Where(b.engine.Zone)
	lines := 0
	if b.system != nil {
		lines = len(sos.Lines(b.system))
	}
	sources := []string{}
	for _, s := range b.engine.Sources() {
		sources = append(sources, s.Name)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"country": country, "numbers": sos.Numbers(country), "lines": lines, "located": located,
		"lat": here.Lat, "lon": here.Lon, "contacts": cfg.Contacts, "medical": cfg.Medical, "sources": sources,
	})
	return string(data), nil
}

func (b *Bus) Call(number string) *dbus.Error {
	if b.system == nil {
		return fail(sos.ErrNoLine)
	}
	country, _, _ := b.engine.Config().Where(b.engine.Zone)
	if _, err := sos.Call(b.system, country, number); err != nil {
		return fail(err)
	}
	return nil
}

func (b *Bus) Changed() {
	if b.session != nil {
		b.session.Emit(Path, Name+".Changed")
	}
}

const xmlNode = `<node><interface name="` + Name + `">
<method name="Alerts"><arg type="s" direction="out"/></method>
<method name="Dismiss"><arg type="s" direction="in"/></method>
<method name="Test"><arg type="s" direction="in"/><arg type="s" direction="out"/></method>
<method name="Config"><arg type="s" direction="out"/></method>
<method name="SetConfig"><arg type="s" direction="in"/></method>
<method name="Areas"><arg type="as" direction="out"/></method>
<method name="Sos"><arg type="s" direction="out"/></method>
<method name="Call"><arg type="s" direction="in"/></method>
<signal name="Changed"/>
</interface>` + introspect.IntrospectDataString + `</node>`

func Serve(session, system *dbus.Conn, path string) (*Engine, error) {
	b := &Bus{session: session, system: system}
	b.engine = NewEngine(path, b.Changed)
	if err := session.Export(b, Path, Name); err != nil {
		return nil, err
	}
	if err := session.Export(introspect.Introspectable(xmlNode), Path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, err
	}
	reply, err := session.RequestName(Name, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, fail(errAlreadyRunning)
	}
	return b.engine, nil
}
