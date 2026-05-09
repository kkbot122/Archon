import json
import os
import time
import logging
from kafka import KafkaConsumer, KafkaProducer
from dotenv import load_dotenv
from pathlib import Path

# Setup Path & Env
current_dir = Path(__file__).resolve().parent
env_path = current_dir.parents[3] / ".env"
load_dotenv(dotenv_path=env_path)

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("AI-Kafka-Worker")

# Kafka Config
KAFKA_BROKER = os.getenv("KAFKA_BROKER", "localhost:9092")
VALIDATION_REQ_TOPIC = "architect.validation.requested"
VALIDATION_COMP_TOPIC = "architect.validation.completed"

def start_worker():
    logger.info(f"🎧 Starting AI Background Worker... Connecting to Kafka at {KAFKA_BROKER}")
    
    # Give Kafka a few seconds to boot if running in Docker Compose
    time.sleep(5)
    
    try:
        consumer = KafkaConsumer(
            VALIDATION_REQ_TOPIC,
            bootstrap_servers=KAFKA_BROKER,
            group_id="ai-brain-auditors",
            value_deserializer=lambda x: json.loads(x.decode('utf-8'))
        )
        
        producer = KafkaProducer(
            bootstrap_servers=KAFKA_BROKER,
            value_serializer=lambda v: json.dumps(v).encode('utf-8')
        )
        
        logger.info(f"✅ Listening for deep-audit requests on '{VALIDATION_REQ_TOPIC}'...")

        for message in consumer:
            payload = message.value
            trace_id = payload.get("trace_id", "unknown")
            manifest = payload.get("manifest", {})
            
            logger.info(f"🕵️‍♂️ [Trace: {trace_id}] Deep audit requested for project: {manifest.get('metadata', {}).get('project_name')}")
            
            # =================================================================
            # 🚧 PLACEHOLDER: The deep AI Audit goes here.
            # For now, we simulate a 5-second deep analysis of the JSON.
            # =================================================================
            logger.info("⏳ Analyzing security, scalability, and cost...")
            time.sleep(5) 
            
            # Formulate the response
            audit_report = {
                "trace_id": trace_id,
                "project_id": payload.get("project_id"),
                "status": "completed",
                "findings": [
                    {"level": "warning", "message": "Redis cache has no eviction policy set."},
                    {"level": "info", "message": "Architecture is highly available."}
                ]
            }
            
            # Send the result back to Kafka (so Go can push it to the React UI)
            producer.send(VALIDATION_COMP_TOPIC, audit_report)
            producer.flush()
            
            logger.info(f"📤 Audit complete. Report published to '{VALIDATION_COMP_TOPIC}'.")

    except Exception as e:
        logger.error(f"❌ Kafka Worker Error: {e}")

if __name__ == '__main__':
    start_worker()