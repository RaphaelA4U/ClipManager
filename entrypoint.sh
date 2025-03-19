#!/bin/bash
php artisan schedule:work &  # Start de scheduler in de achtergrond
exec "$@"  # Start php-fpm