package helpers

import "siaga-api/api/contracts"

var app *contracts.App

func Init(a *contracts.App) {
	app = a
}
