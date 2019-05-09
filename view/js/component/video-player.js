var template = `
<div>
    <video ref="videoPlayer" class="cygnus-video video-js">
        <source :src=source type="application/x-mpegURL">
        <p class="vjs-no-js">
            To view this video please enable JavaScript, and consider upgrading to a web browser that
            <a href="https://videojs.com/html5-video-support/" target="_blank">supports HTML5 video</a>
        </p>
    </video>
</div>`

export default {
    template: template,
    props: {
        source: String,
        options: {
            type: Object,
            default () {
                return {};
            }
        }
    },
    data() {
        return {
            player: null
        }
    },
    mounted() {
        this.player = videojs(this.$refs.videoPlayer, {
            controls: true,
            preload: "auto",
            autoplay: true,
            muted: true,
            html5: {
                hls: { overrideNative: true }
            },
        });
    },
    beforeDestroy() {
        if (this.player) {
            this.player.dispose()
        }
    }
}