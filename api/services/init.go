package services

import (
	"siaga-api/api/contracts"
	"siaga-api/api/services/admin"
	"siaga-api/api/services/auth"
	"siaga-api/api/services/satpam"
)

func Init(app *contracts.App) *contracts.Services {
	srv := &contracts.Services{
		Auth:   auth.Init(app),
		Satpam: satpam.Init(app),
		Admin:  admin.Init(app),
	}

	app.Logger.Log().Msg("Initializing Services: Pass")

	return srv
}
