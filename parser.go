package main

import(
	"fmt"
	"encoding/json/v2"
	"os"
	"io"
)

type JSONKeybinds struct{
	JSONKeybinds []JSONKeybind `json:"keybinds"`
}

// Representation of keybinds.go Keybind struct in keybinds.json


type JSONKeybind struct {
	Key string `json:"Key"`
	Action string `json:"Action"`
	Args []string `json:"Args"`
}

func (wm *WM) parseKeybinds() ([]Keybind, error){
	keybinds := make([]Keybind,0)

	HOME, err := os.UserHomeDir()
	if err != nil{
		return keybinds, err
	}

	dir := fmt.Sprintf("%s/keybinds.json", HOME) 

	fd, err := os.Open(dir)
	if err != nil{
		return keybinds, err
	}
	defer fd.Close()

	bytes, err := io.ReadAll(fd)
	if err != nil {
		return keybinds, err
	}

	var arr JSONKeybinds

	err = json.Unmarshal(bytes, &arr)
	if err != nil{
		return keybinds, err
	}

	for _, keybind := range arr.JSONKeybinds{
		keybinds = append(keybinds, Keybind{
			keybind.Key,
			func(){
				wm.spawn(keybind.Action, keybind.Args...)
			},
		})
	}

	
	return keybinds, nil
}
