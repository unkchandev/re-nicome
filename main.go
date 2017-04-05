package main

import (
	"bufio"
	"io/ioutil"
	"log"
	"os"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"gopkg.in/yaml.v2"
)

type MyMainWindow struct {
	*walk.MainWindow
	settings Setting

	nameEdit *walk.LineEdit
	fromEdit *walk.LineEdit
	toEdit   *walk.LineEdit
	nameBox  *walk.ComboBox
}

type Item struct {
	Name string `yaml:"Name"`
	From string `yaml:"From"`
	To   string `yaml:"To"`
}
type Setting struct {
	Items []*Item `yaml:"Items"`
}

func (mw *MyMainWindow) getSettings() []*Item {
	buf, err := ioutil.ReadFile("config.yml")
	if err != nil {
		return nil
	}

	log.Println(string(buf))
	var s Setting
	if err := yaml.Unmarshal(buf, &s); err != nil {
		return nil
	}
	log.Println(s)
	mw.settings = s
	return s.Items
}

func (mw *MyMainWindow) setSettings() {
	for i, v := range mw.settings.Items {
		if v.Name == mw.nameBox.Text() {
			mw.fromEdit.SetText(mw.settings.Items[i].From)
			mw.toEdit.SetText(mw.settings.Items[i].To)
			return
		}
	}
}

func main() {
	mw := new(MyMainWindow)

	if _, err := (MainWindow{
		AssignTo: &mw.MainWindow,
		Title:    "RegExp Replace",
		MinSize:  Size{400, 200},
		Layout:   VBox{},
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
						AssignTo: &mw.fromEdit,
					},

					Label{
						Text: "Replace with:",
					},
					LineEdit{
						AssignTo: &mw.toEdit,
					},

					Label{
						Text: "Settings:",
					},
					ComboBox{
						DisplayMember: "Name",
						Model:         mw.getSettings(),
						OnCurrentIndexChanged: mw.setSettings,
						AssignTo:              &mw.nameBox,
					},
				},
			},
			PushButton{
				Text:      "Save Settings",
				OnClicked: func() { mw.saveSettingsDialog(mw.MainWindow) },
			},
		},
	}.Run()); err != nil {
		log.Fatal(err)
	}
}

func (mw *MyMainWindow) saveSettingsDialog(owner walk.Form) (int, error) {
	if mw.fromEdit.Text() == "" || mw.toEdit.Text() == "" {
		mw.errorDialog(owner, "Invalid value.")
	}

	var dlg *walk.Dialog
	var acceptPB, cancelPB *walk.PushButton

	return Dialog{
		AssignTo:      &dlg,
		Title:         "Set setting name",
		DefaultButton: &acceptPB,
		CancelButton:  &cancelPB,
		MinSize:       Size{300, 100},
		Layout:        VBox{},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{
						Text: "Name:",
					},
					LineEdit{
						AssignTo: &mw.nameEdit,
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{
						AssignTo: &acceptPB,
						Text:     "OK",
						OnClicked: func() {
							if mw.nameEdit.Text() == "" {
								mw.errorDialog(owner, "Invalid name.")
								return
							} else {
								mw.saveSettings()
								mw.nameBox.SetModel(mw.settings.Items)
								dlg.Accept()
								return
							}
						},
					},
					PushButton{
						AssignTo:  &cancelPB,
						Text:      "Cancel",
						OnClicked: func() { dlg.Cancel() },
					},
				},
			},
		},
	}.Run(owner)
}

func (mw *MyMainWindow) errorDialog(owner walk.Form, msg string) (int, error) {
	var dlg *walk.Dialog
	return Dialog{
		AssignTo: &dlg,
		Title:    "Error",
		MinSize:  Size{200, 80},
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: VBox{},
				Children: []Widget{
					Label{
						Text: msg,
					},
					PushButton{
						Text: "OK",
						OnClicked: func() {
							dlg.Accept()
							return
						},
					},
				},
			},
		},
	}.Run(owner)
}

func (mw *MyMainWindow) saveSettings() {
	var item = Item{
		mw.nameEdit.Text(),
		mw.fromEdit.Text(),
		mw.toEdit.Text(),
	}
	var flag bool = true
	for i, v := range mw.settings.Items {
		if v.Name == item.Name {
			flag = false
			mw.settings.Items[i].From = item.From
			mw.settings.Items[i].To = item.To
			break
		}
	}
	if flag {
		mw.settings.Items = append(mw.settings.Items, &item)
	}

	fp, err := os.OpenFile("config.yml", os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()
	writer := bufio.NewWriter(fp)
	yml, err := yaml.Marshal(mw.settings)
	if err != nil {
		log.Fatal(err)
	}
	_, err = writer.WriteString(string(yml))
	if err != nil {
		log.Fatal(err)
	}
	writer.Flush()
}
