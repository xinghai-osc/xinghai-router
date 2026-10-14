package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

const dataTransferTermsVersion = "13f038a8387ce0cb5b6ea31ea3d1e564e0a945a1ef4135d73968fd0254459b5a"

const dataTransferTermsDocument = "使用需要数据出入境条款的模型前，你同意相关请求内容可能被传输至中华人民共和国境外的模型服务商及其处理设施，并按照服务商的条款处理。请确认你已获得提交相关数据所需的授权，并避免提交不应跨境传输的个人信息、商业秘密或其他受保护数据。"

type dataTransferConsent struct {
	TermsVersion string     `json:"terms_version"`
	AcceptedAt   *time.Time `json:"accepted_at"`
}

type dataTransferConsentResponse struct {
	TermsVersion string     `json:"terms_version"`
	Document     string     `json:"document"`
	Accepted     bool       `json:"accepted"`
	AcceptedAt   *time.Time `json:"accepted_at"`
}

type dataTransferConsentRequest struct {
	TermsVersion string `json:"terms_version"`
}

func (s *Service) getDataTransferConsent(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	consent := dataTransferConsent{}
	err := s.db.QueryRow(r.Context(), `select terms_version, accepted_at from account_data_transfer_consents where user_id=$1`, account.userID).Scan(&consent.TermsVersion, &consent.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, dataTransferConsentResponse{TermsVersion: dataTransferTermsVersion, Document: dataTransferTermsDocument})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	writeJSON(w, http.StatusOK, dataTransferConsentResponse{TermsVersion: dataTransferTermsVersion, Document: dataTransferTermsDocument, Accepted: consent.TermsVersion == dataTransferTermsVersion, AcceptedAt: consent.AcceptedAt})
}

func (s *Service) putDataTransferConsent(w http.ResponseWriter, r *http.Request) {
	var input dataTransferConsentRequest
	if decode(r, &input) != nil || input.TermsVersion != dataTransferTermsVersion {
		writeError(w, http.StatusBadRequest, "invalid_request", "terms_version is invalid")
		return
	}
	account := accountFromContext(r)
	var consent dataTransferConsent
	err := s.db.QueryRow(r.Context(), `
		insert into account_data_transfer_consents (user_id, terms_version, accepted_at)
		values ($1, $2, now())
		on conflict (user_id) do update set terms_version=excluded.terms_version, accepted_at=excluded.accepted_at
		returning terms_version, accepted_at`, account.userID, input.TermsVersion).Scan(&consent.TermsVersion, &consent.AcceptedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save data transfer consent")
		return
	}
	writeJSON(w, http.StatusOK, dataTransferConsentResponse{TermsVersion: consent.TermsVersion, Document: dataTransferTermsDocument, Accepted: true, AcceptedAt: consent.AcceptedAt})
}

func (s *Service) requiresDataTransferConsent(ctx context.Context, userID, model string) (bool, error) {
	var required bool
	err := s.db.QueryRow(ctx, `select exists(select 1 from model_catalog_metadata where model=$1 and requires_data_transfer_consent) and not exists(select 1 from account_data_transfer_consents where user_id=$2 and terms_version=$3)`, model, userID, dataTransferTermsVersion).Scan(&required)
	return required, err
}
