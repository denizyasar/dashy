curl -i \
 --header "Content-Type: application/json" \
 --header "Authorization: Bearer secret" \
 --data @testdata.json http://127.0.0.1:8080/wavedata
curl -i \
 --header "Content-Type: application/json" \
 --header "Authorization: Bearer secret" \
 --data @testdata2.json http://127.0.0.1:8080/wavedata
