package main

import (
	"fmt"
	"github.com/beevik/ntp"
	"os"
)

//Создать программу, печатающую точное текущее время с использованием NTP-сервера.
//
//Реализовать проект как модуль Go.
//
//Использовать библиотеку ntp для получения времени.
//
//Программа должна выводить текущее время, полученное через NTP (Network Time Protocol).
//
//Необходимо обрабатывать ошибки библиотеки: в случае ошибки вывести её текст в STDERR и вернуть ненулевой код выхода.
//
//Код должен проходить проверки (vet и golint), т.е. быть написан идиоматически корректно.

func main() {
	currentTime, err := ntp.Time("0.pool.ntp.org")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ntp.Time: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("CurrentTime:", currentTime)
}
