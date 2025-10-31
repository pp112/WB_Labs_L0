## Старт

Видео: https://cloud.mail.ru/public/y9g6/K1HnqaBe1

#### 1. Клонируйте репозиторий:

```bash
git clone https://github.com/pp112/WB_Labs_L0.git
cd WB_Labs_L0/
```

#### 2. Запустите сервисы через Docker Compose:

```bash
docker compose up --build -d
```

#### 3. Сделайте скрипт отправки заказов исполняемым:

```bash
chmod +x send_order.sh
```

#### 4. Откройте веб-интерфейс:

Перейдите в браузере по адресу:  
http://localhost:8080/ui

#### 5. Добавьте заказ:

- Через скрипт:

```bash
./send_order.sh /path/to/file_or_folder
```

- Или напрямую через контейнер publisher:

```bash
docker compose exec publisher go run /app/publish.go /path/to/file_or_folder
```

> Файл должен быть в формате JSON. Можно указывать как один файл, так и папку с файлами.

#### 6. Проверка заказа:

- Введите `order_uid` созданного заказа в веб-интерфейсе:  
http://localhost:8080/ui

---

## Структура

- `cmd/service/main.go` — точка входа order-service  
- `internal/` — бизнес-логика, DB, NATS consumer, кэш  
- `tools/publisher/publish.go` — скрипт для отправки заказов через NATS  
- `web/index.html` — интерфейс для проверки заказов  
