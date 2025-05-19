package sessions

import "vesta/handlers"

func CreateTeam(Client handlers.Client, session string) {
	sesh := handlers.Sessions[session]
	if sesh == nil {
		return
	}

}

