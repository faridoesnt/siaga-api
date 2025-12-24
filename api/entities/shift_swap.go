package entities

import "time"

type ShiftSwapStatus string

const (
	ShiftSwapStatusPending  ShiftSwapStatus = "PENDING"
	ShiftSwapStatusApproved ShiftSwapStatus = "APPROVED"
	ShiftSwapStatusRejected ShiftSwapStatus = "REJECTED"
)

type ShiftSwapRequest struct {
	ID                    int64            `db:"id" json:"id"`
	RequesterUserID       int64            `db:"requester_user_id" json:"requester_user_id"`
	TargetUserID          int64            `db:"target_user_id" json:"target_user_id"`
	RequesterName         *string          `db:"requester_name" json:"requester_name,omitempty"`
	TargetName            *string          `db:"target_name" json:"target_name,omitempty"`
	ShiftDate             time.Time        `db:"shift_date" json:"shift_date"`
	RequesterUserShiftID  int64            `db:"requester_user_shift_id" json:"requester_user_shift_id"`
	TargetUserShiftID     int64            `db:"target_user_shift_id" json:"target_user_shift_id"`
	Status                ShiftSwapStatus  `db:"status" json:"status"`
	Reason                *string          `db:"reason" json:"reason,omitempty"`
	Note                  *string          `db:"note" json:"note,omitempty"`
	DecidedBy             *int64           `db:"decided_by" json:"decided_by,omitempty"`
	DecidedAt             *time.Time       `db:"decided_at" json:"decided_at,omitempty"`
	CreatedAt             time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time        `db:"updated_at" json:"updated_at"`
}
