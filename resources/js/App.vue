<template>
    <v-app>
        <NavBar />
        <v-main>
            <v-container>
                <HelloWorld msg="Hello, World!" />
            </v-container>
        </v-main>
    </v-app>
</template>

<script>
import NavBar from './components/NavBar.vue';
import HelloWorld from './components/HelloWorld.vue';

export default {
    name: 'App',
    components: {
        NavBar,
        HelloWorld,
    },
};
</script>

<style>
@import 'vuetify/styles';
</style>