FROM php:8.2-fpm

# Installeer PHP-dependencies en FFmpeg
RUN apt-get update && apt-get install -y \
    libzip-dev unzip git ffmpeg \
    && docker-php-ext-install pdo_mysql zip

# Installeer Node.js en npm
RUN curl -fsSL https://deb.nodesource.com/setup_18.x | bash - \
    && apt-get install -y nodejs \
    && npm install -g npm

WORKDIR /var/www
COPY ./app .

# Installeer Composer en PHP-dependencies
RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer \
    && composer install --no-dev --optimize-autoloader

# Installeer Node-dependencies en bouw de frontend
RUN npm ci \
    && npm run build

# Stel rechten in voor Laravel storage en cache
RUN chown -R www-data:www-data /var/www/storage /var/www/bootstrap/cache \
    && chmod -R 775 /var/www/storage /var/www/bootstrap/cache

# Entrypoint voor scheduler
COPY entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh
ENTRYPOINT ["entrypoint.sh"]
CMD ["php-fpm"]