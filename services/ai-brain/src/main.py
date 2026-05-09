import threading
import logging
from grpcserver.server import serve
from grpcserver.worker import start_worker

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("AI-Brain-Main")

if __name__ == '__main__':
    logger.info("🚀 Booting AI Brain Service...")
    
    # Start the Kafka consumer in a background daemon thread
    worker_thread = threading.Thread(target=start_worker, daemon=True)
    worker_thread.start()
    
    # Start the gRPC server on the main thread (this blocks and keeps the app alive)
    serve()