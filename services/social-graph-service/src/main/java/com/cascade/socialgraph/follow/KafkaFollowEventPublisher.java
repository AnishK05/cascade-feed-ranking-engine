package com.cascade.socialgraph.follow;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.ObjectMapper;

@Component
public class KafkaFollowEventPublisher implements FollowEventPublisher {

  private static final Logger log = LoggerFactory.getLogger(KafkaFollowEventPublisher.class);

  private final KafkaTemplate<String, String> kafkaTemplate;
  private final ObjectMapper objectMapper;
  private final String topic;

  public KafkaFollowEventPublisher(
      KafkaTemplate<String, String> kafkaTemplate,
      ObjectMapper objectMapper,
      @Value("${cascade.follow-events-topic}") String topic) {
    this.kafkaTemplate = kafkaTemplate;
    this.objectMapper = objectMapper;
    this.topic = topic;
  }

  @Override
  public void publish(FollowEvent event) {
    final String json;
    try {
      json = objectMapper.writeValueAsString(event);
    } catch (JacksonException exception) {
      throw new IllegalStateException("Failed to serialize " + event.eventType(), exception);
    }
    kafkaTemplate
        .send(topic, Long.toString(event.followeeId()), json)
        .whenComplete(
            (result, exception) -> {
              if (exception != null) {
                log.error(
                    "Failed to publish {} for followee {}",
                    event.eventType(),
                    event.followeeId(),
                    exception);
              }
            });
  }
}
