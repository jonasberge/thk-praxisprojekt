COMPOSE=docker-compose.test.yml

container_start:
	sudo docker-compose -f $(COMPOSE) up -d --build --force-recreate

.PHONY: container_start
