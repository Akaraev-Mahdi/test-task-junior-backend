package task

import "time"

type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// Добавляем типы периодичности
type RecurrenceType string

const (
	RecurrenceDaily    RecurrenceType = "daily"    // ежедневные (каждый n-й день)
	RecurrenceMonthly  RecurrenceType = "monthly"  // ежемесячные (число месяца)
	RecurrenceDates    RecurrenceType = "dates"    // конкретные даты
	RecurrenceEvenOdd  RecurrenceType = "even_odd" // четные/нечетные
)

// RecurrenceSettings описывает правила повторения задачи
type RecurrenceSettings struct {
	Type       RecurrenceType `json:"type"`                  
	Interval   int            `json:"interval,omitempty"`    // n-й день
	DayOfMonth int            `json:"day_of_month,omitempty"` // число месяца (1-30)
	Dates      []time.Time    `json:"dates,omitempty"`        // конкретные даты
	IsEven     *bool          `json:"is_even,omitempty"`      // true - четные, false - нечетные
}

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Recurrence *RecurrenceSettings `json:"recurrence,omitempty"`
	IsTemplate  bool      `json:"is_template"`
	ParentId 	*int64     `json:"parent_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

func (r *RecurrenceSettings) ShouldCreateToday(createdAt time.Time, now time.Time) bool {
	if r == nil {
		return false
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
    startDay := time.Date(createdAt.Year(), createdAt.Month(), createdAt.Day(), 0, 0, 0, 0, createdAt.Location())

	switch r.Type {
	case RecurrenceDaily:
        if r.Interval <= 0 {
            return false
        }

        daysDiff := int(today.Sub(startDay).Hours() / 24)
        
        return daysDiff >= 0 && daysDiff%r.Interval == 0

	case RecurrenceMonthly:
		return now.Day() == r.DayOfMonth

	case RecurrenceEvenOdd:
		isEven := now.Day()%2 == 0
		if r.IsEven != nil {
			return isEven == *r.IsEven
		}

	case RecurrenceDates:
		for _, d := range r.Dates {
			if d.Year() == now.Year() && d.YearDay() == now.YearDay() {
				return true
			}
		}
	}
	return false
}

