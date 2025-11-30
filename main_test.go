package main

import (
	"os"
	"testing"
	"time" // <-- ДОБАВЬТЕ ЭТОТ ИМПОРТ
)

func TestPersistence(t *testing.T) {
	tmpFile := "test_data.json"
	defer os.Remove(tmpFile)

	// 1. Создаем хранилище
	p := NewPersistence(tmpFile)
	userID := int64(12345)

	// 2. Проверяем изменение состояния
	p.SetState(userID, StateChoosing)
	if got := p.GetState(userID); got != StateChoosing {
		t.Errorf("Ожидал StateChoosing, получил %v", got)
	}

	// 3. Проверяем запись данных
	p.UpdateData(userID, "Name", "Ivan")
	p.UpdateData(userID, "Age", "30")

	dataStr := p.GetDataString(userID)
	if dataStr == "Нет данных." {
		t.Error("Данные не сохранились")
	}
	
	time.Sleep(10 * time.Millisecond)

	// 4. Симулируем перезапуск бота (создаем новый инстанс, читающий тот же файл)
	p2 := NewPersistence(tmpFile)
	
	// Проверяем, что данные восстановились из файла
	if p2.GetState(userID) != StateChoosing {
		t.Error("Состояние не восстановилось после перезагрузки")
	}
	
	restoredData := p2.GetDataString(userID)
	if restoredData != dataStr {
		t.Errorf("Данные восстановились некорректно. Ожидал:\n%s\nПолучил:\n%s", dataStr, restoredData)
	}
}