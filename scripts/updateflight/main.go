package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	flightv1 "flightbooking/internal/flightv1"
	"flightbooking/internal/svcjwt"
)

func main() {
	addr := flag.String("addr", "localhost:9090", "адрес gRPC Flight")
	secret := flag.String("secret", "", "FLIGHT_JWT_SECRET")
	id := flag.String("id", "", "id рейса")
	price := flag.String("price", "", "цена")
	statusName := flag.String("status", "", "SCHEDULED, DEPARTED, CANCELLED или COMPLETED")
	departure := flag.String("departure", "", "время вылета, RFC3339")
	arrival := flag.String("arrival", "", "время прилёта, RFC3339")
	flag.Parse()
	if *secret == "" || *id == "" || *price == "" || *statusName == "" || *departure == "" || *arrival == "" {
		flag.Usage()
		os.Exit(2)
	}
	flightStatus, err := parseStatus(*statusName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	departureAt, err := time.Parse(time.RFC3339, *departure)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	arrivalAt, err := time.Parse(time.RFC3339, *arrival)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	conn, err := grpc.NewClient(
		*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(svcjwt.UnaryClientInterceptor(*secret)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := flightv1.NewFlightServiceClient(conn).UpdateFlight(ctx, &flightv1.UpdateFlightRequest{
		Id:          *id,
		Price:       *price,
		Status:      flightStatus,
		DepartureAt: timestamppb.New(departureAt),
		ArrivalAt:   timestamppb.New(arrivalAt),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s %s %s\n", resp.GetPrice(), resp.GetStatus().String(), resp.GetDepartureAt().AsTime().UTC().Format(time.RFC3339))
}

func parseStatus(value string) (flightv1.FlightStatus, error) {
	switch value {
	case "SCHEDULED":
		return flightv1.FlightStatus_FLIGHT_STATUS_SCHEDULED, nil
	case "DEPARTED":
		return flightv1.FlightStatus_FLIGHT_STATUS_DEPARTED, nil
	case "CANCELLED":
		return flightv1.FlightStatus_FLIGHT_STATUS_CANCELLED, nil
	case "COMPLETED":
		return flightv1.FlightStatus_FLIGHT_STATUS_COMPLETED, nil
	default:
		return flightv1.FlightStatus_FLIGHT_STATUS_UNSPECIFIED, fmt.Errorf("неизвестный статус %s", value)
	}
}
