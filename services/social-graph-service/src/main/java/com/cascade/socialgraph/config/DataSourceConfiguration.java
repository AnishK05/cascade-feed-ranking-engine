package com.cascade.socialgraph.config;

import java.net.URI;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import javax.sql.DataSource;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import org.springframework.util.StringUtils;

@Configuration
public class DataSourceConfiguration {

  @Bean
  DataSource dataSource(Environment environment) {
    String databaseUrl = environment.getProperty("DATABASE_URL");
    String username =
        environment.getProperty(
            "DB_USER", environment.getProperty("spring.datasource.username", "postgres"));
    String password =
        environment.getProperty(
            "DB_PASSWORD", environment.getProperty("spring.datasource.password", "postgres"));

    if (!StringUtils.hasText(databaseUrl)) {
      databaseUrl = environment.getProperty("spring.datasource.url");
    }
    if (!StringUtils.hasText(databaseUrl)) {
      throw new IllegalArgumentException("A database URL is required");
    } else if (databaseUrl.startsWith("postgres://")
        || databaseUrl.startsWith("postgresql://")) {
      ParsedDatabaseUrl parsed = parseDatabaseUrl(databaseUrl);
      databaseUrl = parsed.jdbcUrl();
      username = parsed.username();
      password = parsed.password();
    }

    return DataSourceBuilder.create()
        .url(databaseUrl)
        .username(username)
        .password(password)
        .build();
  }

  private ParsedDatabaseUrl parseDatabaseUrl(String databaseUrl) {
    URI uri = URI.create(databaseUrl);
    String[] credentials = uri.getRawUserInfo() == null ? new String[0] : uri.getRawUserInfo().split(":", 2);
    if (credentials.length != 2) {
      throw new IllegalArgumentException("DATABASE_URL must include username and password");
    }
    int port = uri.getPort() < 0 ? 5432 : uri.getPort();
    String query = uri.getRawQuery() == null ? "" : "?" + uri.getRawQuery();
    String jdbcUrl = "jdbc:postgresql://" + uri.getHost() + ":" + port + uri.getRawPath() + query;
    return new ParsedDatabaseUrl(
        jdbcUrl, decode(credentials[0]), decode(credentials[1]));
  }

  private String decode(String value) {
    return URLDecoder.decode(value, StandardCharsets.UTF_8);
  }

  private record ParsedDatabaseUrl(String jdbcUrl, String username, String password) {}
}
