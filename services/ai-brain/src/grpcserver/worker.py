import json
import os
import time
import logging
from kafka import KafkaConsumer, KafkaProducer
from kafka.errors import NoBrokersAvailable
from dotenv import load_dotenv
from pathlib import Path

# Setup Path & Env
current_dir = Path(__file__).resolve().parent
env_path = current_dir.parents[3] / ".env"
load_dotenv(dotenv_path=env_path)

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("AI-Kafka-Worker")

# Kafka Config from Environment
KAFKA_BROKER = os.getenv("KAFKA_BROKER", "localhost:9092")
KAFKA_GROUP = os.getenv("KAFKA_CONSUMER_GROUP", "ai-brain-auditors")
VALIDATION_REQ_TOPIC = "architect.validation.requested"
VALIDATION_COMP_TOPIC = "architect.validation.completed"

def connect_with_retry(retries=5, backoff=5):
    """Attempts to connect to Kafka with an exponential backoff loop."""
    for attempt in range(retries):
        try:
            logger.info(f"🔌 Connecting to Kafka at {KAFKA_BROKER} (Attempt {attempt + 1}/{retries})...")
            
            consumer = KafkaConsumer(
                VALIDATION_REQ_TOPIC,
                bootstrap_servers=KAFKA_BROKER,
                group_id=KAFKA_GROUP,
                api_version=(3, 5, 0),
                auto_offset_reset='earliest', # FIX: Process missed messages on restart
                enable_auto_commit=True,
                value_deserializer=lambda x: json.loads(x.decode('utf-8'))
            )
            
            producer = KafkaProducer(
                bootstrap_servers=KAFKA_BROKER,
                api_version=(3, 5, 0),
                value_serializer=lambda v: json.dumps(v).encode('utf-8')
            )
            
            return consumer, producer
            
        except NoBrokersAvailable:
            logger.warning(f"⚠️ Kafka not ready. Retrying in {backoff} seconds...")
            time.sleep(backoff)
            
    raise Exception("❌ Max retries reached. Could not connect to Kafka.")

def start_worker():
    try:
        # FIX: Replaced sleep(5) with a robust retry loop
        consumer, producer = connect_with_retry()
        logger.info(f"✅ Listening for deep-audit requests on '{VALIDATION_REQ_TOPIC}'...")

        for message in consumer:
            payload = message.value
            trace_id = payload.get("trace_id", "unknown")
            manifest = payload.get("manifest", {})
            
            logger.info(f"🕵️‍♂️ [Trace: {trace_id}] Deep audit requested for project: {manifest.get('metadata', {}).get('project_name')}")
            
            # Simulated AI Audit...
            logger.info("⏳ Analyzing security, scalability, and cost...")
            time.sleep(2) 
            
            audit_report = {
                "trace_id": trace_id,
                "project_id": payload.get("project_id"),
                "status": "completed",
                "findings": [
                    {"level": "warning", "message": "Redis cache has no eviction policy set."},
                    {"level": "info", "message": "Architecture is highly available."}
                ]
            }
            
            producer.send(VALIDATION_COMP_TOPIC, audit_report)
            producer.flush()
            logger.info(f"📤 Audit complete. Report published to '{VALIDATION_COMP_TOPIC}'.")

    except Exception as e:
        logger.error(f"❌ Kafka Worker Error: {e}")

if __name__ == '__main__':
    start_worker()