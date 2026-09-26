# max-vpn — быстрый старт

Нужно: macOS 14+, Apple Silicon, Go, Node 20+.

## 1. Установить и запустить

```sh
make pkg && sudo installer -pkg build/max-vpn.pkg -target /
open /Applications/max-vpn.app
```

Эти команды соберут приложение и демон, поставят их в систему (демон
будет стартовать сам при загрузке) и откроют приложение.

## 2. Импортировать конфиг

**Locations**, дальше одно из двух:

- **WireGuard**: **Import file**, выбери `.conf` (или перетащи файл в окно).
- **VLESS**: вставь ссылку `vless://…` в поле и нажми **Add**.

## 3. Подключиться

Нажми на имя сервера в **Locations** или **Connect** на главном экране.

Проверить, что работает:

```sh
curl https://api.ipify.org   # должен показать IP VPN-сервера
```

---

## Удалить

```sh
make uninstall-daemon && sudo rm -rf /Applications/max-vpn.app
```

## Если пропал интернет

```sh
sudo pfctl -a com.apple/maxvpn -F all            # снять kill switch
sudo networksetup -setdnsservers Wi-Fi Empty     # вернуть авто-DNS
```
