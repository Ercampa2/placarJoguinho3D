#!/bin/sh
# Registra um gole de água no bot. Associe este arquivo a um atalho de teclado
# (veja "Sip scripts" no README). Precisa do curl; o aviso na tela usa o
# notify-send (pacote libnotify) e, sem ele, a resposta sai no terminal.

BOT_URL="http://192.168.0.10:8080"   # endereço da máquina do bot
TOKEN="COLE_AQUI_SEU_TOKEN"          # gere com /agua_token no Discord
TRILHAS="5min"                       # "5min", "10min" ou "5min 10min"

for trilha in $TRILHAS; do
	# Sem -f, o curl devolve o texto do bot também nas recusas (ex.: "já
	# registrou"); só falha quando não consegue falar com o bot.
	resposta=$(curl -sS --max-time 5 -X POST \
		-H "Authorization: Bearer $TOKEN" "$BOT_URL/gole/$trilha" 2>&1) ||
		resposta="Sem resposta do bot em $BOT_URL ($resposta)"

	if command -v notify-send >/dev/null 2>&1; then
		notify-send "Gole de água ($trilha)" "$resposta"
	else
		echo "$trilha: $resposta"
	fi
done
