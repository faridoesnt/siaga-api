package handlers

import (
	. "siaga-api/api/contracts"
)

var app *App

func Init(a *App) {
	app = a
}
