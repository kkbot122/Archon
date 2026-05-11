import threading
import logging
from grpc_server.server import serve
from grpc_server.worker import start_worker

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("AI-Brain-Main")

if __name__ == '__main__':
    logger.info("🚀 Booting AI Brain Service...")
    
    stop_event = threading.Event()
    
    # Start the Kafka consumer in a background thread
    worker_thread = threading.Thread(target=start_worker, args=(stop_event,), daemon=True)
    worker_thread.start()
    
    try:
        # Start the gRPC server on the main thread (blocks)
        serve()
    finally:
        logger.info("🛑 Main thread shutting down. Signalling background worker...")
        stop_event.set()
        worker_thread.join(timeout=10)