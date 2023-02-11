service_generated_path=internal/generated/rpc

swagger_out_path=swaggerui

gen.service:
	protoc -I ./api/service --go_out=$(service_generated_path) \
		  	--go-grpc_out=$(service_generated_path) \
		  	--grpc-gateway_out=$(service_generated_path) \
		  	--openapiv2_out=$(swagger_out_path) \
			 ./api/service/*.proto

gen.clients:
	./gen-clients.sh

