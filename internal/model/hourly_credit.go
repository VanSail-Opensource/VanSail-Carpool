package model

import (
	"errors"
	"time"
)

var ErrHourlyCreditNotFound = errors.New("hourly credit reservation not found")
var ErrHourlyCreditUnavailable = errors.New("hourly byte credit is not available")
var ErrHourlyCreditExpired = errors.New("hourly credit day is not current UTC day")
var ErrHourlyCreditNotReserved = errors.New("hourly credit definitely not reserved")

// HourlyCreditDay is an internal durable projection, not a live rate grant.
// Every byte count is raw ingress upload plus download, without multipliers.
// Wire/API adapters select approved fields and encode large integers as strings.
type HourlyCreditDay struct {
	OwnerID            string
	AccountGeneration  int64
	DayUTC             string
	FreeAllowanceBytes int64
	FreeAdmittedBytes  int64
	FreeReservedBytes  int64
	ExtraFundedBytes   int64
	ExtraAdmittedBytes int64
	ExtraReservedBytes int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// FreeAvailableBytes excludes both confirmed consumption and outstanding grants.
func (d HourlyCreditDay) FreeAvailableBytes() int64 {
	return d.FreeAllowanceBytes - d.FreeAdmittedBytes - d.FreeReservedBytes
}

// ExtraAvailableBytes can use only credit already funded by commerce.
func (d HourlyCreditDay) ExtraAvailableBytes() int64 {
	return d.ExtraFundedBytes - d.ExtraAdmittedBytes - d.ExtraReservedBytes
}

// HourlyCreditRequest is the immutable scope selected before issuing an agent
// installation. OwnerID is the existing users.id, without identity conversion.
type HourlyCreditRequest struct {
	ReservationID     string
	OwnerID           string
	AccountGeneration int64
	DayUTC            string
	NodeID            string
	AgentID           string
	BootID            string
	SessionID         string
	Kind              string
	ByteCeiling       int64
}

// HourlyCreditReservation retains the obligation through disconnect, expiry,
// purge and acknowledgement. Confirmed counters are monotonic projections;
// the original final proof digest/recovery flag remain immutable.
type HourlyCreditReservation struct {
	HourlyCreditRequest
	IssueSequence         int64
	AdmittedUploadBytes   int64
	AdmittedDownloadBytes int64
	UnusedBytes           int64
	UnknownBytes          int64
	FinalProofDigest      string
	Recovery              bool
	CreatedAt             time.Time
	FinalizedAt           *time.Time
}

// ReservedBytes includes every byte not confirmed admitted or proven unused.
func (r HourlyCreditReservation) ReservedBytes() int64 {
	return r.ByteCeiling - r.AdmittedUploadBytes - r.AdmittedDownloadBytes - r.UnusedBytes
}

// HourlyCreditProof describes the original agent proof. Recovery currently
// reports all bytes unknown; stronger saved/live progress is combined locally.
type HourlyCreditProof struct {
	HourlyCreditRequest
	IssueSequence         int64
	AdmittedUploadBytes   int64
	AdmittedDownloadBytes int64
	UnusedBytes           int64
	UnknownBytes          int64
	Recovery              bool
	Digest                string
}

// HourlyCreditProgress carries monotonic, directional admitted-byte maxima.
type HourlyCreditProgress struct {
	ReservationID         string
	IssueSequence         int64
	AdmittedUploadBytes   int64
	AdmittedDownloadBytes int64
}
