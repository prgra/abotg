package main

import (
	"log"

	"github.com/BurntSushi/toml"
	"github.com/prgra/abotg/abot"
)

func main() {
	var c abot.Conf
	_, err := toml.DecodeFile("config.toml", &c)
	if err != nil {
		log.Println(err)
	}

	err = abot.Run(c)
	if err != nil {
		log.Println(err)
	}
}
