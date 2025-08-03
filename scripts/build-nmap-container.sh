#!/bin/bash

# Script to build and manage the NetWeather NMAP scanner container
# Usage: ./build-nmap-container.sh [build|start|stop|status|logs]

set -e

CONTAINER_NAME="netweather-nmap-scanner"
IMAGE_NAME="netweather-nmap"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

case "${1:-build}" in
    "build")
        echo "Building NetWeather NMAP scanner container..."
        cd "$PROJECT_DIR"
        
        # Build for both architectures if buildx is available
        if docker buildx version &>/dev/null; then
            echo "Building multi-architecture image..."
            docker buildx build --platform linux/amd64,linux/arm64 \
                -t "$IMAGE_NAME" ./docker/nmap-scanner --load
        else
            echo "Building single architecture image..."
            docker build -t "$IMAGE_NAME" ./docker/nmap-scanner
        fi
        
        echo "✓ Container built successfully: $IMAGE_NAME"
        ;;
        
    "start")
        echo "Starting NetWeather NMAP scanner container..."
        
        # Stop existing container if running
        if docker ps -q -f name="$CONTAINER_NAME" | grep -q .; then
            echo "Stopping existing container..."
            docker stop "$CONTAINER_NAME"
        fi
        
        # Remove existing container if exists
        if docker ps -aq -f name="$CONTAINER_NAME" | grep -q .; then
            echo "Removing existing container..."
            docker rm "$CONTAINER_NAME"
        fi
        
        # Start new container
        docker run -d \
            --name "$CONTAINER_NAME" \
            -p 8080:8080 \
            --restart unless-stopped \
            "$IMAGE_NAME"
        
        echo "✓ Container started: $CONTAINER_NAME"
        echo "Waiting for service to be ready..."
        
        # Wait for health check
        for i in {1..30}; do
            if curl -s http://localhost:8080/health &>/dev/null; then
                echo "✓ NMAP scanner service is ready!"
                break
            fi
            if [ $i -eq 30 ]; then
                echo "✗ Service failed to start within 30 seconds"
                docker logs "$CONTAINER_NAME"
                exit 1
            fi
            sleep 1
        done
        ;;
        
    "stop")
        echo "Stopping NetWeather NMAP scanner container..."
        if docker ps -q -f name="$CONTAINER_NAME" | grep -q .; then
            docker stop "$CONTAINER_NAME"
            echo "✓ Container stopped: $CONTAINER_NAME"
        else
            echo "Container not running: $CONTAINER_NAME"
        fi
        ;;
        
    "status")
        echo "NetWeather NMAP Scanner Status:"
        echo "================================"
        
        if docker ps -q -f name="$CONTAINER_NAME" | grep -q .; then
            echo "Status: Running"
            echo "Container ID: $(docker ps -q -f name="$CONTAINER_NAME")"
            echo "Health: $(curl -s http://localhost:8080/health 2>/dev/null || echo "Unhealthy")"
        elif docker ps -aq -f name="$CONTAINER_NAME" | grep -q .; then
            echo "Status: Stopped"
        else
            echo "Status: Not created"
        fi
        
        echo ""
        if docker images -q "$IMAGE_NAME" | grep -q .; then
            echo "Image: Available ($IMAGE_NAME)"
            docker images "$IMAGE_NAME" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
        else
            echo "Image: Not built"
        fi
        ;;
        
    "logs")
        echo "NetWeather NMAP scanner logs:"
        echo "============================="
        if docker ps -aq -f name="$CONTAINER_NAME" | grep -q .; then
            docker logs "$CONTAINER_NAME" "${@:2}"
        else
            echo "Container not found: $CONTAINER_NAME"
        fi
        ;;
        
    "restart")
        echo "Restarting NetWeather NMAP scanner container..."
        "$0" stop
        sleep 2
        "$0" start
        ;;
        
    "clean")
        echo "Cleaning up NetWeather NMAP scanner..."
        
        # Stop and remove container
        if docker ps -aq -f name="$CONTAINER_NAME" | grep -q .; then
            docker stop "$CONTAINER_NAME" 2>/dev/null || true
            docker rm "$CONTAINER_NAME" 2>/dev/null || true
            echo "✓ Container removed"
        fi
        
        # Remove image
        if docker images -q "$IMAGE_NAME" | grep -q .; then
            docker rmi "$IMAGE_NAME"
            echo "✓ Image removed"
        fi
        
        echo "✓ Cleanup complete"
        ;;
        
    "help"|*)
        echo "NetWeather NMAP Container Management"
        echo "===================================="
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  build     Build the NMAP scanner container image"
        echo "  start     Start the NMAP scanner container"
        echo "  stop      Stop the NMAP scanner container"
        echo "  restart   Restart the NMAP scanner container"
        echo "  status    Show container and service status"
        echo "  logs      Show container logs"
        echo "  clean     Remove container and image"
        echo "  help      Show this help message"
        echo ""
        echo "Examples:"
        echo "  $0 build              # Build the container"
        echo "  $0 start              # Start the service"
        echo "  $0 logs -f            # Follow logs"
        echo "  $0 status             # Check status"
        ;;
esac