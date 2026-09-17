# All-Ok-Radius-Server
### Радиус-сервер для обработки DHCP-запросов. Используется для работы с DHCP-серверами микротика.

## Возможности    
#### ***API***    
* Получение данных через API (требуется реализация API, примеры запрос-ответ ниже)
* Работа со списком источников API (для резервирования и балансировки)   
* Проверка работоспособности и отключение неработающих API на определенные время     
* Кеширование ответов API (для уменьшения нагрузки и резервирования на случай недоступности всех API)
* Radreply и PostAuth запросы в API
* Accounting requests

#### ***Radius***     
* Чтение и передача в API следующих параметров: 
   * NAS-Identifier - Имя микротика    
   * NAS-IP-Address  - IP микротика    
   * User-Name - Мак-адрес пользователя    
   * Calling-Station-Id    
   * Called-Station-Id - Имя dhcp-сервера    
   * Agent-Remote-Id   
   * Agent-Circuit-Id    
* Парсинг Circuit-Id, Remote-Id (option82) и передача на апи 
   в виде remote_id, vlan_id, module, port. На данный момент поддерживается только оборудование от D-Link
* Радиус может выдавать пул или конкретный ip-адрес c указанием времени жизни лиза.    
#### Changelog
Изменения можно просмотреть здесь - [CHANGELOG.md](CHANGELOG.md)

#### ***Встроенные Lua-скрипты***
* Обработка auth/accounting/post-auth запросов встроенными Lua-скриптами вместо HTTP API 
   (по аналогии с lua-nginx-module в nginx или rlm_python во FreeRADIUS)    
* На каждый метод - свой скрипт (`script.auth`/`script.acct`/`script.post_auth`), так же как у API свой список `addresses` на каждый метод    
* Скрипты компилируются один раз при старте и выполняются в пуле изолированных состояний,
   без сетевых обращений и без внешних процессов   
* Выбор обработчика единый (`processor: api` или `script`) - или всё через HTTP API, или всё через скрипт;
   пакеты api и script полностью независимы друг от друга    

#### ***Другое***
* Гибкое конфигурирование кеширования и актуализации работы с API    
* Поддержка передачи метрик в формате Prometheus (описание метрик смотрите в экспортере)
 
## Работа с API (Radreply)
Используется для получения информации о выдачи необходимого IP адреса. Данный метод должен возвращать 
**Сервер отправляет POST-запрос с Content-Type: application/json.**
* Пример запроса с передаваемой опцией82: 
```  
{
    "nas_ip": "10.0.0.1",
    "nas_name": "MikroTik-Radius",
    "device_mac": "00:01:02:03:04:05",
    "dhcp_server_name": "DHCP-TEST-101",
    "dhcp_server_id": "1:14:da:e9:a2:7f:7b",
    "agent": {
        "circuit_id": {
           "vlan_id": 101,
           "module": 0,
           "port": 3
        },
        "remote_id": "00:AD:24:0D:F7:B6",
        "_raw_circuit_id": "03f20005"
     }
}
```     
* Пример запроса без option82:     
```  
{
    "nas_ip": "10.0.0.1",
    "nas_name": "MikroTik-Radius",
    "device_mac": "00:01:02:03:04:05",
    "dhcp_server_name": "DHCP-TEST-101",
    "dhcp_server_id": "1:14:da:e9:a2:7f:7b",
    "agent": null
}
```

* Пример запроса с опцией, но ошибкой парсинга circuitId:     
```  
{
    "nas_ip": "10.0.0.1",
    "nas_name": "MikroTik-Radius",
    "device_mac": "00:01:02:03:04:05",
    "dhcp_server_name": "DHCP-TEST-101",
    "dhcp_server_id": "1:14:da:e9:a2:7f:7b",
    "agent": {
        "circuit_id": null,
        "remote_id": "00:AD:24:0D:F7:B6",
        "_raw_circuit_id": "03f20005"
    }
}
```
**Ответ от API должен быть в следующем формате**    
* Выдача пула с таймаутом лиза 2 минуты     
``` 
{
    "statusCode": 200,
    "data": {
        "pool_name": "INET-FAKE-101",
        "lease_time_sec": 120
    }
}
```    
* Выдача IP-адреса с таймаутом лиза на 1ч     
``` 
{
    "statusCode": 200,
    "data": {
        "ip_address": "172.16.3.233",
        "lease_time_sec": 3600
    }
}
```     

## Работа с API (PostAuth)     
**Сервер отправляет POST-запрос с Content-Type: application/json.**    
Пример запроса сервера:    
```
{
    "request": {
         "nas_ip": "<nil>",
         "nas_name": "", 
         "device_mac": "AA:BB:CC:DD:EE:FF",
         "dhcp_server_name":"vlan1244", "dhcp_server_id": "",
         "agent":null
    },
    "response": {
         "ip_address": "", 
         "pool_name": "vlan1244", 
         "lease_time_sec": 120,
         "status": "ACCEPT",
         "error":""
    }
}
```   
Радиус не анализирует ответ от API    


## Работа с API (Accounting)     
**Сервер отправляет POST-запрос с Content-Type: application/json.**    
Структура:    
```
struct {
	string `json:"nas_ip" 
	string `json:"nas_name"  
	string `json:"device_mac"`
	string `json:"dhcp_server_name"`
	string `json:"dhcp_server_id"`
	string `json:"ip_address"`
	string `json:"auth_type"`
	string `json:"class_id"`
	string `json:"status_type"`
	int64  `json:"session_time"`
	string `json:"terminate_cause"`
	int64  `json:"input_octets"`
	int64  `json:"output_octets"`
	string `json:"pool_name"`
	string `json:"session_id"`
}

```   
Радиус не анализирует ответ от API    

## Встроенные Lua-скрипты (альтернатива HTTP API)
Вместо HTTP API запросы можно обрабатывать встроенными Lua-скриптами - они выполняются прямо в
процессе радиус-сервера, без сетевых обращений, по аналогии с lua-nginx-module в nginx или
rlm_python во FreeRADIUS. Каждый скрипт компилируется один раз при старте и затем выполняется в
пуле из `script.pool_size` изолированных состояний (по одному на одновременный вызов).

Это выбор **или/или** на верхнем уровне конфига: `processor: api` или `processor: script` -
методы не могут использовать разные бэкенды одновременно. При этом пакеты `api` и `script` ничего
не знают друг о друге - это две независимые реализации одного интерфейса, между ними выбирает
только `server/main.go` по значению `processor`.

Для каждого метода - свой Lua-файл, так же как у HTTP API свой список `addresses` на каждый метод:
`script.auth` (обязателен), `script.acct` и `script.post_auth` (опциональны - если путь не задан,
метод просто не обрабатывается скриптом). Можно указать один и тот же файл для нескольких методов.

**Важно:** сейчас вся бизнес-логика (парсинг circuit_id/remote_id из option82, поиск клиента по
MAC/агенту и т.п.) реализована на стороне HTTP API. При переходе на `processor: script` скрипт
получает те же сырые данные, что раньше уходили в API (см. `option.remote_id`, `option.circuit_id`
в request ниже) - её нужно реализовать в скрипте самостоятельно, HTTP API при этом не участвует.

Примеры скриптов - [script/examples/auth.lua](script/examples/auth.lua), [acct.lua](script/examples/acct.lua), [post_auth.lua](script/examples/post_auth.lua).

```yaml
processor: script
script:
  pool_size: 10
  timeout: 3s
  auth: /etc/radius/scripts/auth.lua           # authorize(request) -> table
  acct: /etc/radius/scripts/acct.lua           # accounting(request)
  post_auth: /etc/radius/scripts/post_auth.lua # post_auth(request, response)
```

Каждый файл может определять только нужную ему функцию - `authorize` для `script.auth`,
`accounting` для `script.acct`, `post_auth` для `script.post_auth`. Если функция отсутствует в
указанном файле - сервер не запустится и явно укажет на это в логе.
* `authorize(request) -> table` - должна вернуть таблицу с `ip_address` или `pool_name`
  (опционально `lease_time_sec`), либо `{error = "..."}` для отказа
* `accounting(request)` - без возвращаемого значения
* `post_auth(request, response)` - уведомление о финальном ответе, отправленном NAS-у, без
  возвращаемого значения

Поля `request` для auth совпадают с телом HTTP-запроса Radreply (см. выше): `nas_ip`, `nas_name`,
`device_mac`, `dhcp_server_name`, `dhcp_server_id`, `ip_address`, `class_id`,
`option.remote_id`, `option.circuit_id`. Поля `request` для accounting/post_auth аналогичны
структурам Accounting/PostAuth выше.

Внутри скрипта доступен глобальный объект `log` (`log.debug/info/notice/warning/error(msg)`) -
пишет в общий лог радиус-сервера.

### Как запустить       
1. Можно использовать докер (описание находится в ./install/docker)    
2. Скачать бинарник с релизов и пример конфига. Можно запустить руками или же добавить в sysctl (описание находится в ./install/deamon)     

### Интеграции c биллингами
* NoDeny - https://github.com/meklis/all-ok-radius-nodeny-lib   
* AllOkBilling   
### Пример настройки микротика для опции82 или без нее 
![Winbox screen](doc/mikrotik_screen.png)
