// Текст ошибки для экрана: сообщение Error, а если выброшена не ошибка (строка, объект) — она сама.
// Раньше в каждом catch стояло своё «String(e.message || e)», а часть мест писала просто e.message и показывала «undefined».
export const errText = (e) => String(e?.message || e)
