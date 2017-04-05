package main

import (
	"io/ioutil"
	"log"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"gopkg.in/yaml.v2"
)

type MyMainWindow struct {
	*walk.MainWindow

	logTE *walk.TextEdit
}

type Item struct {
	Name string `yaml:"Name"`
	From string `yaml:"From"`
	To   string `yaml:"To"`
}
type Setting struct {
	Items []*Item `yaml:"Items"`
}

func getSettings() []*Item {
	buf, err := ioutil.ReadFile("config.yml")
	if err != nil {
		return nil
	}

	var s Setting
	if err := yaml.Unmarshal(buf, &s); err != nil {
		return nil
	}
	return s.Items
}

func main() {
	if _, err := (MainWindow{
		Title:   "RegExp Replace",
		MinSize: Size{10, 30},
		Layout:  VBox{},
		OnDropFiles: func(files []string) {
		},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{
						Text: "Find what:",
					},
					LineEdit{
						Text: Bind("From"),
					},

					Label{
						Text: "Replace with:",
					},
					LineEdit{
						Text: Bind("To"),
					},

					Label{
						Text: "Settings:",
					},
					ComboBox{
						Value:         Bind("Setting", SelRequired{}),
						DisplayMember: "Name",
						Model:         getSettings(),
					},
				},
			},
		},
	}.Run()); err != nil {
		log.Fatal(err)
	}
}

func (mw *MyMainWindow) openAction_Triggered() {
}
