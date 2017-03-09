go build
docker-machine scp tapcon-monitor smaster-1:
docker-machine scp config.json smaster-1:
docker-machine scp tapcon-monitor sworker-1:
docker-machine scp config.json sworker-1:
