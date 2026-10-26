package sc

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"
)

// cryXML builds a CryXmlB file for <root a="1"><child b="two"/></root>.
func cryXML() []byte {
	strs := []byte{}
	off := map[string]uint32{}
	s := func(v string) uint32 {
		if o, ok := off[v]; ok {
			return o
		}
		off[v] = uint32(len(strs))
		strs = append(strs, append([]byte(v), 0)...)
		return off[v]
	}
	le := binary.LittleEndian
	node := func(name string, attrs, kids uint16, parent, firstAttr, firstChild int32) []byte {
		b := make([]byte, 28)
		le.PutUint32(b[0:], s(name))
		le.PutUint16(b[8:], attrs)
		le.PutUint16(b[10:], kids)
		le.PutUint32(b[12:], uint32(parent))
		le.PutUint32(b[16:], uint32(firstAttr))
		le.PutUint32(b[20:], uint32(firstChild))
		return b
	}
	nodes := append(node("root", 1, 1, -1, 0, 0), node("child", 1, 0, 0, 1, 0)...)
	attrs := make([]byte, 16)
	le.PutUint32(attrs[0:], s("a"))
	le.PutUint32(attrs[4:], s("1"))
	le.PutUint32(attrs[8:], s("b"))
	le.PutUint32(attrs[12:], s("two"))
	kids := make([]byte, 4)
	le.PutUint32(kids, 1)
	head := make([]byte, 44)
	copy(head, "CryXmlB\x00")
	nodeOff := 44
	attrOff := nodeOff + len(nodes)
	childOff := attrOff + len(attrs)
	strOff := childOff + len(kids)
	for i, v := range []int{strOff + len(strs), nodeOff, 2, attrOff, 2, childOff, 1, strOff, len(strs)} {
		le.PutUint32(head[8+4*i:], uint32(v))
	}
	return bytes.Join([][]byte{head, nodes, attrs, kids, strs}, nil)
}

func TestCryXMLToText(t *testing.T) {
	out, err := CryXMLToText(cryXML())
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `<root a="1"><child b="two"/></root>` {
		t.Fatalf("got %s", out)
	}
	if _, err := CryXMLToText([]byte("<root/>")); err == nil {
		t.Fatal("plain XML accepted as CryXmlB")
	}
}

const profile = `<ActionMaps>
 <actionmap name="spaceship_general" UILabel="@ui_general">
  <action name="v_toggle_landing_system" UILabel="@ui_gear" activationMode="press" keyboard="n"/>
  <action name="v_hail" UILabel="@ui_hail"><keyboard input=" "/></action>
  <action name="v_unlabeled"/>
 </actionmap>
 <actionmap name="debug" UILabel="Debug">
  <action name="d_thing" UILabel="Thing" keyboard="f1"/>
 </actionmap>
</ActionMaps>`

func TestParseProfile(t *testing.T) {
	loc := ParseLocalization([]byte("\ufeffui_general=General\nui_gear,P=Landing gear\nui_hail=Hail\n"))
	acts, err := ParseProfile([]byte(profile), loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 2 {
		t.Fatalf("want 2 labeled actions outside debug, got %+v", acts)
	}
	if acts[0].Label != "Landing gear" || acts[0].Key != "n" || acts[0].MapLabel != "General" {
		t.Fatalf("gear: %+v", acts[0])
	}
	if acts[1].Key != "" {
		t.Fatalf("hail should be unbound: %+v", acts[1])
	}
}

func TestBindAllKeepsPlayerBinds(t *testing.T) {
	acts := []Action{
		{Map: "spaceship_general", Name: "gear", Label: "Gear", Key: "n"},
		{Map: "spaceship_general", Name: "hail", Label: "Hail"},
		{Map: "spaceship_general", Name: "wave", Label: "Wave"},
		{Map: "vehicle_general", Name: "wave", Label: "Wave"},
		{Map: "spaceship_general", Name: "lights", Label: "Lights", Key: "l"},
		{Map: "spaceship_general", Name: "look", Label: "Look", Key: "mouse3"},
	}
	// The shape of a real actionmaps.xml: a HOTAS, device options, one keyboard rebind
	// and one cleared bind.
	current := []byte(`<ActionMaps>
 <ActionProfiles version="1" optionsVersion="2" rebindVersion="2" profileName="default">
  <options type="keyboard" instance="1" Product="Keyboard"/>
  <options type="joystick" instance="1" Product=" VKB L">
   <flight_strafe_longitudinal invert="1"/>
  </options>
  <options type="joystick" instance="3"/>
  <modifiers />
  <actionmap name="spaceship_general">
   <action name="gear">
    <rebind input="kb1_m"/>
   </action>
   <action name="hail">
    <rebind input="js1_button6" multiTap="2"/>
   </action>
   <action name="lights">
    <rebind input="kb1_ "/>
   </action>
  </actionmap>
 </ActionProfiles>
</ActionMaps>`)
	out, added := BindAll(acts, current, "Every action", "ralt+a")
	if len(added) != 3 || added["hail"] == "" || added["wave"] == "" || added["look"] == "" || added["lights"] != "" {
		t.Fatalf("want hail, wave and the mouse-only look bound, cleared lights left alone: %v", added)
	}
	if added["hail"] == "ralt+a" {
		t.Fatal("used a reserved combination")
	}
	x := string(out)
	for _, want := range []string{`profileName="Every action"`, `kb1_m`, `js1_button6" multiTap="2"`, `kb1_` + added["hail"],
		`<flight_strafe_longitudinal invert="1"/>`, `<joystick instance="1"/>`, `<actionmap name="vehicle_general">`} {
		if !strings.Contains(x, want) {
			t.Fatalf("profile is missing %s:\n%s", want, x)
		}
	}
	if strings.Contains(x, `<joystick instance="3"/>`) {
		t.Fatal("listed a device that isn't connected")
	}
	if strings.Count(x, `<action name="hail">`) != 1 || strings.Count(x, `name="wave"`) != 2 {
		t.Fatalf("actions duplicated or missing:\n%s", x)
	}
	if strings.Count(x, "kb1_"+added["wave"]) != 2 {
		t.Fatal("the same action in two maps should get the same key")
	}
	// It still parses as XML.
	d := xml.NewDecoder(strings.NewReader(x))
	for {
		if _, err := d.Token(); err != nil {
			if err.Error() != "EOF" {
				t.Fatalf("not valid XML: %v\n%s", err, x)
			}
			break
		}
	}
	for _, k := range added {
		if strings.Contains(k, "f4") || k == "m" || k == "n" {
			t.Fatalf("bad key %s", k)
		}
	}
}
