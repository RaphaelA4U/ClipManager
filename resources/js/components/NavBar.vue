<template>
    <v-app-bar app>
      <v-toolbar-title>Pooltafel Monitor</v-toolbar-title>
      <v-spacer></v-spacer>
      <v-btn color="primary" @click="clip">Clip</v-btn>
    </v-app-bar>
  </template>
  
<script>
  export default {
    name: 'NavBar',

    methods: {
      clip() {
        fetch('/clip', { method: 'POST' });
      },
    },
  };
</script>  