package clientdb

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BindEvent - точечное изменение одной записи в bind-источнике (см. Config.Binds),
// приходит через Redis pub/sub в дополнение к периодическому полному reload по HTTP.
// В отличие от reload, применяется без перезагрузки всего источника - патчит по
// месту только запись с данным ID, остальные записи не затрагиваются.
type BindEvent struct {
	DB     string `json:"db"`     // имя источника, как в Config.Binds (например "clients")
	Action string `json:"action"` // "upsert" или "delete"
	ID     string `json:"id"`     // обязателен для action=delete
	Line   string `json:"line"`   // "id;ip;client_mac[;device_mac;port]" - формат строки как у HTTP-источника (см. loadBindDB), обязателен для action=upsert
}

// handleRedisMessage - точка входа для сообщений из Redis pub/sub (см. subscribeRedis).
func (s *Store) handleRedisMessage(payload string) {
	var ev BindEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		s.lg.ErrorF("clientdb: live-update: невалидный json от redis: %v", err)
		return
	}
	if err := s.ApplyBindEvent(ev); err != nil {
		s.lg.ErrorF("clientdb: live-update: %v", err)
	}
}

// ApplyBindEvent применяет точечное изменение (см. BindEvent) к уже загруженному
// источнику binds по имени ev.DB, не затрагивая остальные записи и не запуская
// полный reload. Источник должен уже существовать в текущем снапшоте (см.
// Config.Binds) - событие для несконфигурированного/ещё не загруженного dbName
// отклоняется с ошибкой, а не создаёт источник "на лету".
func (s *Store) ApplyBindEvent(ev BindEvent) error {
	idx, ok := s.currentSnapshot().binds[ev.DB]
	if !ok {
		return fmt.Errorf("live-update для несуществующего источника binds.%v", ev.DB)
	}

	switch ev.Action {
	case "delete":
		if ev.ID == "" {
			return fmt.Errorf("live-update: delete без id (binds.%v)", ev.DB)
		}
		idx.deleteByID(ev.ID)
		return nil
	case "upsert":
		b, ok := parseBindFields(strings.Split(strings.TrimSpace(ev.Line), ";"))
		if !ok {
			return fmt.Errorf("live-update: не удалось разобрать line (binds.%v): %q", ev.DB, ev.Line)
		}
		if ev.ID != "" && ev.ID != b.ID {
			return fmt.Errorf("live-update: id в событии (%v) не совпадает с id в line (%v) (binds.%v)", ev.ID, b.ID, ev.DB)
		}
		idx.upsert(b)
		return nil
	default:
		return fmt.Errorf("live-update: неизвестный action %q (binds.%v)", ev.Action, ev.DB)
	}
}
