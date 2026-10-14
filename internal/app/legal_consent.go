package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type legalConsent struct {
	TermsVersion   string     `json:"terms_version"`
	PrivacyVersion string     `json:"privacy_version"`
	AcceptedAt     *time.Time `json:"accepted_at"`
}

type legalConsentRequest struct {
	TermsVersion   string `json:"terms_version"`
	PrivacyVersion string `json:"privacy_version"`
}

func (s *Service) getLegalConsent(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	consent := legalConsent{}
	err := s.db.QueryRow(r.Context(), `select terms_version, privacy_version, accepted_at from account_legal_consents where user_id=$1`, account.userID).Scan(&consent.TermsVersion, &consent.PrivacyVersion, &consent.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, consent)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	writeJSON(w, http.StatusOK, consent)
}

func (s *Service) putLegalConsent(w http.ResponseWriter, r *http.Request) {
	var input legalConsentRequest
	if err := decode(r, &input); err != nil || !validLegalConsentVersion(input.TermsVersion) || !validLegalConsentVersion(input.PrivacyVersion) {
		writeError(w, http.StatusBadRequest, "invalid_request", "terms_version and privacy_version must be lowercase sha256 hashes")
		return
	}
	account := accountFromContext(r)
	var consent legalConsent
	err := s.db.QueryRow(r.Context(), `
		insert into account_legal_consents (user_id, terms_version, privacy_version, accepted_at)
		values ($1, $2, $3, now())
		on conflict (user_id) do update set terms_version=excluded.terms_version, privacy_version=excluded.privacy_version, accepted_at=excluded.accepted_at
		returning terms_version, privacy_version, accepted_at`, account.userID, input.TermsVersion, input.PrivacyVersion).Scan(&consent.TermsVersion, &consent.PrivacyVersion, &consent.AcceptedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save legal consent")
		return
	}
	writeJSON(w, http.StatusOK, consent)
}

func validLegalConsentVersion(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
