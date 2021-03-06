package keyset

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/uol/gobol/rip"
)

// CreateKeySet - creates a new keyset
func (ks *KeySet) CreateKeySet(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	keySetParam := ps.ByName("keyset")

	if keySetParam == "" {
		rip.AddStatsMap(r, map[string]string{"path": "/keysets/#keyset", "keyset": "empty"})
		rip.Fail(w, errBadRequest("CreateKeySet", "parameter 'keyset' cannot be empty"))
		return
	}

	if !ks.keySetRegexp.MatchString(keySetParam) {
		rip.AddStatsMap(r, map[string]string{"path": "/keysets/#keyset"})
		rip.Fail(w, errBadRequest("CreateKeySet", "parameter 'keyset' has an invalid format"))
		return
	}

	rip.AddStatsMap(r, map[string]string{"path": "/keysets/#keyset", "keyset": keySetParam})

	exists, gerr := ks.storage.CheckKeySet(keySetParam)
	if gerr != nil {
		rip.Fail(w, gerr)
		return
	}

	if exists {
		rip.Success(w, http.StatusConflict, nil)
	} else {
		gerr := ks.CreateIndex(keySetParam)
		if gerr != nil {
			rip.Fail(w, gerr)
			return
		}
		rip.Success(w, http.StatusCreated, nil)
	}

	return
}

// GetKeySets - returns all stored keysets
func (ks *KeySet) GetKeySets(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	keysets, gerr := ks.storage.ListKeySets()

	if gerr != nil {
		rip.AddStatsMap(r, map[string]string{"path": "/keysets"})
		rip.Fail(w, errInternalServerError("GetKeySets", gerr))
		return
	}

	if keysets == nil || len(keysets) == 0 {
		rip.SuccessJSON(w, http.StatusNoContent, nil)
	} else {
		rip.SuccessJSON(w, http.StatusOK, keysets)
	}

	return
}
